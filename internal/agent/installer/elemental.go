package installer

import (
	"fmt"
	"os"

	"github.com/mudler/yip/pkg/schema"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/elementalcli"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/twpayne/go-vfs"
	"gopkg.in/yaml.v3"
)

var _ Installer = (*ElementalInstaller)(nil)

type ElementalInstaller struct {
	fs          vfs.FS
	cliRunner   elementalcli.Runner
	cmdRunner   utils.CommandRunner
	hostManager host.Manager
	configPath  string
	workDir     string
}

func NewElementalInstaller(fs vfs.FS, hostManager host.Manager, configPath string, workDir string) Installer {
	return &ElementalInstaller{
		fs:          fs,
		cliRunner:   elementalcli.NewRunner(),
		cmdRunner:   utils.NewCommandRunner(),
		hostManager: hostManager,
		configPath:  configPath,
		workDir:     workDir,
	}
}

func (i *ElementalInstaller) Install(conf api.RegistrationResponse, hostnameToSet string) error {
	log.Debug("Installing Elemental")
	// Unmarshal the remote install config
	install := &elementalcli.Install{}
	if err := unmarshalRaw(conf.Config.Elemental.Install, install); err != nil {
		return fmt.Errorf("unmarshalling install config: %w", err)
	}
	// Creating temporary files
	writtenFiles, err := i.createCloudInitTemporaryFiles(conf, hostnameToSet)
	if err != nil {
		return fmt.Errorf("creating temporary cloud-init files: %w", err)
	}
	install.ConfigURLs = append(install.ConfigURLs, writtenFiles...)
	// Set the Hostname on current environment
	if err := i.hostManager.SetHostname(hostnameToSet); err != nil {
		return fmt.Errorf("setting hostname: %w", err)
	}
	log.Infof("Hostname set: %s", hostnameToSet)
	// Install
	log.Info("Running elemental install")
	if err := i.cliRunner.Install(*install); err != nil {
		return fmt.Errorf("running elemental install: %w", err)
	}
	return nil
}

func (i *ElementalInstaller) createCloudInitTemporaryFiles(conf api.RegistrationResponse, hostnameToSet string) ([]string, error) {
	writtenFiles := []string{}
	// Create /tmp dir if not exists yet
	if err := utils.CreateDirectory(i.fs, temporaryDir); err != nil {
		return nil, fmt.Errorf("creating temporary dir: %w", err)
	}
	// Write temporary remote cloud-init config
	cloudInitPath := fmt.Sprintf("%s/%s", temporaryDir, cloudInitFile)
	cloudInitBytes, err := formatCloudConfig(conf.Config.CloudConfig)
	if err != nil {
		return nil, fmt.Errorf("formatting cloud-init config: %w", err)
	}
	if err := i.fs.WriteFile(cloudInitPath, cloudInitBytes, os.ModePerm); err != nil {
		return nil, fmt.Errorf("writing temporary cloud init config: %w", err)
	}
	writtenFiles = append(writtenFiles, cloudInitPath)
	// Write host config with agent conf file and hostname
	hostConfigPath := fmt.Sprintf("%s/%s", temporaryDir, hostConfigFile)
	agentConfig := config.FromAPI(conf)
	agentConfigBytes, err := yaml.Marshal(agentConfig)
	if err != nil {
		return nil, fmt.Errorf("marshalling agent config: %w", err)
	}
	hostNameCommand := fmt.Sprintf("echo %s > /etc/hostname", hostnameToSet)
	hostNameConfig := schema.YipConfig{
		Name: "Configure host",
		Stages: map[string][]schema.Stage{
			"boot.after": {
				{
					Commands: []string{hostNameCommand},
					Files: []schema.File{
						{
							Path:        fmt.Sprintf(i.configPath),
							Content:     string(agentConfigBytes),
							Permissions: 0600,
						},
					},
				},
			},
		},
	}
	hostNameConfigBytes, err := yaml.Marshal(hostNameConfig)
	if err != nil {
		return nil, fmt.Errorf("marshalling hostname config: %w", err)
	}
	if err := i.fs.WriteFile(hostConfigPath, hostNameConfigBytes, os.ModePerm); err != nil {
		return nil, fmt.Errorf("writing temporary host config: %w", err)
	}
	writtenFiles = append(writtenFiles, hostConfigPath)
	return writtenFiles, nil
}

func (i *ElementalInstaller) TriggerReset() error {
	log.Debug("Triggering Elemental reset")
	// Create /oem dir if not exists yet.
	if err := utils.CreateDirectory(i.fs, "/oem"); err != nil {
		return fmt.Errorf("creating oem dir: %w", err)
	}
	// This is the local cloud-config that the elemental-agent will run while in recovery mode
	resetCloudConfig := schema.YipConfig{
		Name: "Elemental Reset",
		Stages: map[string][]schema.Stage{
			"network.after": {
				schema.Stage{
					If:   "[ -f /run/cos/recovery_mode ]",
					Name: "Runs elemental reset and reinstall the system",
					Commands: []string{
						"cp /oem/elemental/agent/config.yaml /tmp/elemental-agent-config.yaml",
						"elemental-agent --debug --reset --config /oem/elemental/agent/config.yaml",
						"mount /oem",
						"mkdir -p /oem/elemental/agent",
						"mv /tmp/elemental-agent-config.yaml /oem/elemental/agent/config.yaml",
						"systemctl start elemental-agent-install.service",
					},
				},
			},
		},
	}
	resetCloudConfigBytes, err := yaml.Marshal(resetCloudConfig)
	if err != nil {
		return fmt.Errorf("marshalling reset cloud config: %w", err)
	}
	resetCloudConfigPath := "/oem/reset-cloud-config.yaml"
	if err := i.fs.WriteFile(resetCloudConfigPath, resetCloudConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing reset cloud config file '%s': %w", resetCloudConfigPath, err)
	}
	log.Info("Setting next default grub entry to recovery")
	if err := i.cmdRunner.RunCommand("grub2-editenv /oem/grubenv set next_entry=recovery"); err != nil {
		return fmt.Errorf("setting next default grub entry to recovery: %w", err)
	}
	log.Info("Scheduling reboot in 1 minute")
	if err := i.cmdRunner.RunCommand("shutdown -r +1"); err != nil {
		return fmt.Errorf("scheduling reboot: %w", err)
	}
	return nil
}

func (i *ElementalInstaller) Reset(conf api.RegistrationResponse) error {
	log.Debug("Resetting Elemental")
	// Unmarshal the remote reset config
	reset := &elementalcli.Reset{}
	if err := unmarshalRaw(conf.Config.Elemental.Reset, reset); err != nil {
		return fmt.Errorf("unmarshalling reset config: %w", err)
	}
	// Reset
	log.Info("Running elemental reset")
	if err := i.cliRunner.Reset(*reset); err != nil {
		return fmt.Errorf("running elemental reset: %w", err)
	}
	return nil
}
