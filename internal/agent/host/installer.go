package host

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

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
	hostnameManager hostname.Manager
	configPath      string
	workDir         string
}

func NewElementalInstaller(fs vfs.FS, hostnameManager hostname.Manager, configPath string, workDir string) Installer {
	return &ElementalInstaller{
		fs:              fs,
		cliRunner:       elementalcli.NewRunner(),
		hostnameManager: hostnameManager,
		configPath:      configPath,
		workDir:         workDir,
	}
}

func (i *ElementalInstaller) Install(_ api.RegistrationResponse, hostnameToSet string) error {
	log.Debug("Installing Elemental")
	// Set the Hostname
	if err := i.hostnameManager.SetHostname(hostnameToSet); err != nil {
		return fmt.Errorf("setting hostname: %w", err)
	}
	log.Infof("Hostname set: %s", hostnameToSet)
	return nil
}

func (i *ElementalInstaller) TriggerReset() error {
	log.Debug("Triggering Elemental reset")
	return nil
}

func (i *ElementalInstaller) Reset(_ api.RegistrationResponse) error {
	log.Debug("Resetting Elemental")
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
	installBytes, err := UnmarshalRawMapToYaml(conf.Config.Elemental.Install)
	if err != nil {
		return fmt.Errorf("unmarshalling install config: %w", err)
	}
	installPath := fmt.Sprintf("%s/%s", conf.Config.Elemental.Agent.WorkDir, installFile)
	if err := utils.WriteFile(i.fs, api.WriteFile{
		Content:     string(installBytes),
		Path:        installPath,
		Owner:       "root:root",
		Permissions: "0640",
	}); err != nil {
		return fmt.Errorf("writing install file to path '%s': %w", installPath, err)
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
	resetBytes, err := UnmarshalRawMapToYaml(conf.Config.Elemental.Reset)
	if err != nil {
		return fmt.Errorf("unmarshalling reset config: %w", err)
	}
	resetPath := fmt.Sprintf("%s/%s", conf.Config.Elemental.Agent.WorkDir, resetFile)
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

func UnmarshalRawMapToYaml(input map[string]runtime.RawExtension) ([]byte, error) {
	bytes := []byte{}
	if len(input) == 0 {
		log.Debug("nothing to decode")
		return bytes, nil
	}
	for key, value := range input {
		var jsonData any
		if err := json.Unmarshal(value.Raw, &jsonData); err != nil {
			return nil, fmt.Errorf("unmarshalling '%s' key with '%s' value: %w", key, string(value.Raw), err)
		}

		yamlData, err := yaml.Marshal(jsonData)
		if err != nil {
			return nil, fmt.Errorf("marshalling '%s' key with '%s' value to yaml: %w", key, string(value.Raw), err)
		}

		bytes = append(bytes, append([]byte(fmt.Sprintf("%s:\n  ", key)), yamlData...)...)
	}

	return bytes, nil
}
