package host

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/mudler/yip/pkg/schema"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/elementalcli"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/hostname"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/twpayne/go-vfs"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	sentinelFileResetNeeded = "reset.needed"
	installFile             = "install.yaml"
	resetFile               = "reset.yaml"
	cloudInitFile           = "cloud-init.yaml"
	hostConfigFile          = "host-config.yaml"
	temporaryDir            = "/tmp"
	installerUnmanaged      = "unmanaged"
	installerElemental      = "elemental"
)

var (
	ErrUnmanagedOSNotReset = errors.New("unmanaged OS reset sentinel file still exists")
	ErrUnknownInstaller    = errors.New("unknown installer")
)

type InstallerSelector interface {
	GetInstaller(fs vfs.FS, configPath string, conf config.Config) (Installer, error)
}

func NewInstallerSelector() InstallerSelector {
	return &installerSelector{}
}

var _ InstallerSelector = (*installerSelector)(nil)

type installerSelector struct{}

func (s *installerSelector) GetInstaller(fs vfs.FS, configPath string, conf config.Config) (Installer, error) {
	var installer Installer
	switch conf.Agent.Installer {
	case installerUnmanaged:
		log.Info("Using Unmanaged OS Installer")
		installer = NewUnmanagedInstaller(fs, hostname.NewManager(), configPath, conf.Agent.WorkDir)
	case installerElemental:
		log.Info("Using Elemental Installer")
		installer = NewElementalInstaller(fs, hostname.NewManager(), configPath, conf.Agent.WorkDir)
	default:
		return nil, fmt.Errorf("parsing installer '%s': %w", conf.Agent.Installer, ErrUnknownInstaller)
	}
	return installer, nil
}

type Installer interface {
	Install(conf api.RegistrationResponse, hostnameToSet string) error
	TriggerReset() error
	Reset(conf api.RegistrationResponse) error
}

var _ Installer = (*ElementalInstaller)(nil)

type ElementalInstaller struct {
	fs              vfs.FS
	cliRunner       elementalcli.Runner
	cmdRunner       utils.CommandRunner
	hostnameManager hostname.Manager
	configPath      string
	workDir         string
}

func NewElementalInstaller(fs vfs.FS, hostnameManager hostname.Manager, configPath string, workDir string) Installer {
	return &ElementalInstaller{
		fs:              fs,
		cliRunner:       elementalcli.NewRunner(),
		cmdRunner:       utils.NewCommandRunner(),
		hostnameManager: hostnameManager,
		configPath:      configPath,
		workDir:         workDir,
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
	if err := i.hostnameManager.SetHostname(hostnameToSet); err != nil {
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
			"initramfs": {
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
						"elemental-agent --debug --reset --config /oem/elemental/agent/config.yaml",
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

var _ Installer = (*UnmanagedInstaller)(nil)

type UnmanagedInstaller struct {
	fs              vfs.FS
	hostnameManager hostname.Manager
	configPath      string
	workDir         string
}

func NewUnmanagedInstaller(fs vfs.FS, hostnameManager hostname.Manager, configPath string, workDir string) Installer {
	return &UnmanagedInstaller{
		fs:              fs,
		hostnameManager: hostnameManager,
		configPath:      configPath,
		workDir:         workDir,
	}
}

func (i *UnmanagedInstaller) Install(conf api.RegistrationResponse, hostnameToSet string) error {
	log.Debug("Installing Unmanaged OS")
	// Write the install config
	installBytes, err := unmarshalRawMapToYaml(conf.Config.Elemental.Install)
	if err != nil {
		return fmt.Errorf("unmarshalling install config: %w", err)
	}
	installPath := fmt.Sprintf("%s/%s", conf.Config.Elemental.Agent.WorkDir, installFile)
	log.Infof("Writing install config file: %s", installPath)
	if err := utils.WriteFile(i.fs, api.WriteFile{
		Content:     string(installBytes),
		Path:        installPath,
		Owner:       "root:root",
		Permissions: "0640",
	}); err != nil {
		return fmt.Errorf("writing install file to path '%s': %w", installPath, err)
	}
	// Write the cloud-init config
	cloudInitBytes, err := formatCloudConfig(conf.Config.CloudConfig)
	if err != nil {
		return fmt.Errorf("formatting cloud-init config: %w", err)
	}
	cloudInitPath := fmt.Sprintf("%s/%s", conf.Config.Elemental.Agent.WorkDir, cloudInitFile)
	if err := utils.WriteFile(i.fs, api.WriteFile{
		Content:     string(cloudInitBytes),
		Path:        cloudInitPath,
		Owner:       "root:root",
		Permissions: "0640",
	}); err != nil {
		return fmt.Errorf("writing cloud-init file to path '%s': %w", installPath, err)
	}
	// Set the Hostname
	if err := i.hostnameManager.SetHostname(hostnameToSet); err != nil {
		return fmt.Errorf("setting hostname: %w", err)
	}
	log.Infof("Hostname set: %s", hostnameToSet)
	// Install the agent config file
	agentConfig := config.FromAPI(conf)
	if err := agentConfig.WriteToFile(i.fs, i.configPath); err != nil {
		return fmt.Errorf("installing agent config: %w", err)
	}
	return nil
}

func (i *UnmanagedInstaller) TriggerReset() error {
	log.Debug("Triggering Unmanaged OS reset")
	sentinelFile := i.formatResetSentinelFile(i.workDir)
	log.Infof("Creating reset sentinel file: %s", sentinelFile)
	if err := utils.WriteFile(i.fs, api.WriteFile{
		Path: sentinelFile,
	}); err != nil {
		return fmt.Errorf("writing reset sentinel file: %w", err)
	}
	return nil
}

func (i *UnmanagedInstaller) Reset(conf api.RegistrationResponse) error {
	log.Debug("Resetting Unmanaged OS")
	// Write the reset config
	resetBytes, err := unmarshalRawMapToYaml(conf.Config.Elemental.Reset)
	if err != nil {
		return fmt.Errorf("unmarshalling reset config: %w", err)
	}
	resetPath := fmt.Sprintf("%s/%s", conf.Config.Elemental.Agent.WorkDir, resetFile)
	log.Infof("Writing reset config file: %s", resetPath)
	if err := utils.WriteFile(i.fs, api.WriteFile{
		Content:     string(resetBytes),
		Path:        resetPath,
		Owner:       "root:root",
		Permissions: "0640",
	}); err != nil {
		return fmt.Errorf("writing reset file to path '%s': %w", resetPath, err)
	}
	// Check reset sentinel file
	sentinelFile := i.formatResetSentinelFile(conf.Config.Elemental.Agent.WorkDir)
	log.Infof("Verifying reset sentinel file '%s' has been deleted", sentinelFile)
	_, err = i.fs.Stat(i.formatResetSentinelFile(conf.Config.Elemental.Agent.WorkDir))
	if err == nil {
		return ErrUnmanagedOSNotReset
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("getting info for file '%s': %w", sentinelFile, err)
	}
	return nil
}

func (i *UnmanagedInstaller) formatResetSentinelFile(workDir string) string {
	return fmt.Sprintf("%s/%s", workDir, sentinelFileResetNeeded)
}

func formatCloudConfig(cloudConfig map[string]runtime.RawExtension) ([]byte, error) {
	cloudInitBytes := []byte("#cloud-config\n")
	cloudInitContentBytes, err := unmarshalRawMapToYaml(cloudConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling cloud init config: %w", err)
	}
	cloudInitBytes = append(cloudInitBytes, cloudInitContentBytes...)
	return cloudInitBytes, nil
}

func unmarshalRawMapToYaml(input map[string]runtime.RawExtension) ([]byte, error) {
	yamlData := []byte{}
	if len(input) == 0 {
		log.Debug("nothing to decode")
		return yamlData, nil
	}

	jsonObject := map[string]any{}
	for key, value := range input {
		var jsonData any
		if err := json.Unmarshal(value.Raw, &jsonData); err != nil {
			return nil, fmt.Errorf("unmarshalling '%s' key with '%s' value: %w", key, string(value.Raw), err)
		}
		jsonObject[key] = jsonData
	}

	yamlData, err := yaml.Marshal(jsonObject)
	if err != nil {
		return nil, fmt.Errorf("marshalling raw json map to to yaml: %w", err)
	}

	return yamlData, nil
}

func unmarshalRaw(input map[string]runtime.RawExtension, output any) error {
	if len(input) == 0 {
		log.Debug("nothing to decode")
		return nil
	}

	jsonBytes := []byte("{")
	for key, value := range input {
		jsonBytes = append(jsonBytes, append([]byte(fmt.Sprintf(`"%s":`, key)), value.Raw...)...)
		jsonBytes = append(jsonBytes, []byte(`,`)...)
	}
	jsonBytes[len(jsonBytes)-1] = byte('}')

	if err := json.Unmarshal(jsonBytes, output); err != nil {
		return fmt.Errorf("unmarshalling json: %w", err)
	}
	return nil
}
