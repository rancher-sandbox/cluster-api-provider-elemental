package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mudler/yip/pkg/schema"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/elementalcli"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/plugin"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	"github.com/twpayne/go-vfs"
	"gopkg.in/yaml.v3"
)

const (
	cloudConfigDir       = "/oem"
	hostnameInitPath     = "/oem/set-hostname.yaml"
	identityInitPath     = "/oem/set-private-key.yaml"
	cloudConfigInitPath  = "/oem/set-cloud-config.yaml"
	agentConfigInitPath  = "/oem/set-config-yaml.yaml"
	agentConfigTempPath  = "/tmp/elemental-agent-config.yaml"
	resetCloudConfigPath = "/oem/reset-cloud-config.yaml"
	liveModeFile         = "/run/cos/live_mode"
)

var _ osplugin.Plugin = (*ElementalPlugin)(nil)

type ElementalPlugin struct {
	fs          vfs.FS
	cliRunner   elementalcli.Runner
	hostManager host.Manager
	cmdRunner   utils.CommandRunner
	workDir     string
	configPath  string
}

func GetPlugin() (osplugin.Plugin, error) {
	return &ElementalPlugin{
		fs:          vfs.OSFS,
		cliRunner:   elementalcli.NewRunner(),
		hostManager: host.NewManager(),
		cmdRunner:   utils.NewCommandRunner(),
	}, nil
}

func (p *ElementalPlugin) Init(context osplugin.PluginContext) error {
	if context.Debug {
		log.EnableDebug()
	}
	log.Debug("Initing Elemental Plugin")
	p.workDir = context.WorkDir
	p.configPath = context.ConfigPath
	if err := utils.CreateDirectory(p.fs, cloudConfigDir); err != nil {
		return fmt.Errorf("creating cloud config directory '%s': %w", cloudConfigDir, err)
	}
	if err := utils.CreateDirectory(p.fs, filepath.Dir(p.configPath)); err != nil {
		return fmt.Errorf("creating config directory '%s': %w", filepath.Dir(p.configPath), err)
	}
	if err := utils.CreateDirectory(p.fs, p.workDir); err != nil {
		return fmt.Errorf("creating work directory '%s': %w", p.workDir, err)
	}
	return nil
}

func (p *ElementalPlugin) InstallCloudInit(input []byte) error {
	log.Debug("Installing cloud-init config")
	cloudInitBytes := []byte("#cloud-config\n")
	cloudInitContentBytes, err := plugin.UnmarshalRawJSONToYaml(input)
	if err != nil {
		return fmt.Errorf("unmarshalling cloud init config: %w", err)
	}
	cloudInitBytes = append(cloudInitBytes, cloudInitContentBytes...)
	if err := p.fs.WriteFile(cloudConfigInitPath, cloudInitBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing cloud init config: %w", err)
	}
	return nil
}

func (p *ElementalPlugin) GetHostname() (string, error) {
	hostname, err := p.hostManager.GetCurrentHostname()
	if err != nil {
		return "", fmt.Errorf("getting current hostname: %w", err)
	}
	return hostname, nil
}

func (p *ElementalPlugin) InstallHostname(hostname string) error {
	log.Debugf("Installing hostname: %s", hostname)
	hostNameCommand := fmt.Sprintf("echo %s > /etc/hostname", hostname)
	hostNameConfig := schema.YipConfig{
		Name: "Configure host",
		Stages: map[string][]schema.Stage{
			"boot.after": {
				{
					Commands: []string{hostNameCommand},
				},
			},
		},
	}
	hostNameConfigBytes, err := yaml.Marshal(hostNameConfig)
	if err != nil {
		return fmt.Errorf("marshalling hostname config: %w", err)
	}
	if err := p.fs.WriteFile(hostnameInitPath, hostNameConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing hostname config in '%s': %w", hostnameInitPath, err)
	}
	return nil
}

func (p *ElementalPlugin) InstallFile(content []byte, path string, permission uint32, owner int, group int) error {
	log.Debugf("Installing file: %s", path)
	// Create a "set-*.yaml" cloud init file to persist the input content
	filename := p.formatSetFileName(path)
	cloudConfigFilePath := fmt.Sprintf("%s/%s", cloudConfigDir, filename)
	writeFileConfig := schema.YipConfig{
		Name: "Write File",
		Stages: map[string][]schema.Stage{
			"boot.after": {
				{
					Files: []schema.File{
						{
							Path:        path,
							Permissions: permission,
							Owner:       owner,
							Group:       group,
							Content:     string(content),
						},
					},
				},
			},
		},
	}
	writeFileConfigBytes, err := yaml.Marshal(writeFileConfig)
	if err != nil {
		return fmt.Errorf("marshalling write file config: %w", err)
	}
	if err := p.fs.WriteFile(cloudConfigFilePath, writeFileConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing hostname config in '%s': %w", cloudConfigFilePath, err)
	}
	return nil
}

// formatSetFileName formats a 'set-*.yaml' filename using the input path
// For ex: 'my-file.foo' --> 'set-my-file-foo.yaml' .
func (p *ElementalPlugin) formatSetFileName(path string) string {
	filenameNoExtension, _ := strings.CutSuffix(filepath.Base(path), filepath.Ext(path))
	extensionNoDot, _ := strings.CutPrefix(filepath.Ext(path), ".")
	return fmt.Sprintf("set-%s-%s.yaml", filenameNoExtension, extensionNoDot)
}

func (p *ElementalPlugin) Install(input []byte) error {
	log.Debug("Installing Elemental")
	// Do not install the system twice.
	// This is the reset scenario where the machine is repurposed instead of reprovisioned from scratch.
	liveMode, err := p.isRunningInLiveMode()
	if err != nil {
		return fmt.Errorf("checking if running in live mode")
	}
	if !liveMode {
		log.Info("Not running from live media. Assuming system is already installed. Nothing to do.")
		return nil
	}
	// Unmarshal the remote install config
	install := elementalcli.Install{}
	if err := json.Unmarshal(input, &install); err != nil {
		return fmt.Errorf("unmarshalling json: %w", err)
	}
	// Include files created during registration
	install.ConfigURLs = append(install.ConfigURLs, hostnameInitPath, identityInitPath, agentConfigInitPath, cloudConfigInitPath)
	// Install
	log.Info("Running elemental install")
	if err := p.cliRunner.Install(install); err != nil {
		return fmt.Errorf("running elemental install: %w", err)
	}
	return nil
}

func (p *ElementalPlugin) Bootstrap(format string, input []byte) error {
	return nil
}

func (p *ElementalPlugin) TriggerReset() error {
	log.Debug("Triggering Elemental reset")
	// Create /oem dir if not exists yet.
	if err := utils.CreateDirectory(p.fs, "/oem"); err != nil {
		return fmt.Errorf("creating oem dir: %w", err)
	}
	// This is the local cloud-config that the elemental-agent will run while in recovery mode
	resetCloudConfig := schema.YipConfig{
		Name: "Elemental Reset",
		Stages: map[string][]schema.Stage{
			"network.after": {
				schema.Stage{
					If:   "[ -f /run/cos/recovery_mode ]",
					Name: "Runs elemental reset and re-register the system",
					Commands: []string{
						"elemental-agent --debug --reset --config /oem/elemental/agent/config.yaml",
						"elemental-agent --debug --register --install --config /oem/elemental/agent/config.yaml",
						"reboot -f",
					},
				},
			},
		},
	}
	resetCloudConfigBytes, err := yaml.Marshal(resetCloudConfig)
	if err != nil {
		return fmt.Errorf("marshalling reset cloud config: %w", err)
	}

	if err := p.fs.WriteFile(resetCloudConfigPath, resetCloudConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing reset cloud config file '%s': %w", resetCloudConfigPath, err)
	}
	log.Info("Setting next default grub entry to recovery")
	if err := p.cmdRunner.RunCommand("grub2-editenv /oem/grubenv set next_entry=recovery"); err != nil {
		return fmt.Errorf("setting next default grub entry to recovery: %w", err)
	}
	log.Info("Scheduling reboot in 1 minute")
	if err := p.cmdRunner.RunCommand("shutdown -r +1"); err != nil {
		return fmt.Errorf("scheduling reboot: %w", err)
	}
	return nil
}

func (p *ElementalPlugin) Reset(input []byte) error {
	log.Debug("Resetting Elemental")
	// Unmarshal the remote reset config
	reset := elementalcli.Reset{}
	if err := json.Unmarshal(input, &reset); err != nil {
		return fmt.Errorf("unmarshalling reset config: %w", err)
	}
	// Copy the current config
	command := fmt.Sprintf("cp %s %s", p.configPath, agentConfigTempPath)
	if err := p.cmdRunner.RunCommand(command); err != nil {
		return fmt.Errorf("running command '%s': %w", command, err)
	}
	// Call elemental-toolkit reset
	log.Info("Running elemental reset")
	if err := p.cliRunner.Reset(reset); err != nil {
		return fmt.Errorf("running elemental reset: %w", err)
	}
	// Mount /oem back if needed
	command = fmt.Sprintf("mount %s", cloudConfigDir)
	if err := p.cmdRunner.RunCommand(command); err != nil {
		return fmt.Errorf("running command '%s': %w", command, err)
	}
	// Create agent config dir if needed
	if err := utils.CreateDirectory(p.fs, filepath.Dir(p.configPath)); err != nil {
		return fmt.Errorf("creating agent config dir '%s': %w", filepath.Dir(p.configPath), err)
	}
	// Restore the config
	command = fmt.Sprintf("mv %s %s", agentConfigTempPath, p.configPath)
	if err := p.cmdRunner.RunCommand(command); err != nil {
		return fmt.Errorf("running command '%s': %w", command, err)
	}
	return nil
}

func (p *ElementalPlugin) PowerOff() error {
	if err := p.hostManager.PowerOff(); err != nil {
		return fmt.Errorf("powering off system: %w", err)
	}
	return nil
}

func (p *ElementalPlugin) Reboot() error {
	if err := p.hostManager.Reboot(); err != nil {
		return fmt.Errorf("rebooting system: %w", err)
	}
	return nil
}

func (p *ElementalPlugin) isRunningInLiveMode() (bool, error) {
	_, err := p.fs.Stat(liveModeFile)
	if err == nil {
		return true, nil
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("getting info for file '%s': %w", liveModeFile, err)
	}
	return false, nil
}
