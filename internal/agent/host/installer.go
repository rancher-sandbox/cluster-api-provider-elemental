package host

import (
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
)

const (
	hostnameFile            = "/etc/hostname"
	sentinelFileResetNeeded = "reset.needed"
)

var (
	ErrManagedOSNotSupportedYet = errors.New("managed Elemental OS not supported yet")
	ErrUnmanagedOSNotReset      = errors.New("unmanaged OS reset sentinel file still exists")
)

type Installer interface {
	Install(conf api.RegistrationResponse, hostnameToSet string) error
	TriggerReset(conf api.RegistrationResponse) error
	Reset(conf api.RegistrationResponse) error
}

var _ Installer = (*ElementalInstaller)(nil)

type ElementalInstaller struct {
	fs        vfs.FS
	cliRunner elementalcli.Runner
}

func NewElementalInstaller(fs vfs.FS) Installer {
	return &ElementalInstaller{
		fs:        fs,
		cliRunner: elementalcli.NewRunner(),
	}
}

func (i *ElementalInstaller) Install(_ api.RegistrationResponse, _ string) error {
	log.Debug("Installing Elemental")
	return ErrManagedOSNotSupportedYet
}

func (i *ElementalInstaller) TriggerReset(_ api.RegistrationResponse) error {
	log.Debug("Triggering Elemental reset")
	return ErrManagedOSNotSupportedYet
}

func (i *ElementalInstaller) Reset(_ api.RegistrationResponse) error {
	log.Debug("Resetting Elemental")
	return ErrManagedOSNotSupportedYet
}

var _ Installer = (*UnmanagedInstaller)(nil)

type UnmanagedInstaller struct {
	fs         vfs.FS
	configPath string
}

func NewUnmanagedInstaller(fs vfs.FS, configPath string) Installer {
	return &UnmanagedInstaller{
		fs:         fs,
		configPath: configPath,
	}
}

func (i *UnmanagedInstaller) Install(conf api.RegistrationResponse, hostnameToSet string) error {
	log.Debug("Installing unmanaged OS.")

	// Set the Hostname
	if err := hostname.SetHostname(hostnameToSet); err != nil {
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

func (i *UnmanagedInstaller) TriggerReset(conf api.RegistrationResponse) error {
	sentinelFile := i.formatResetSentinelFile(conf.Config.Elemental.Agent.WorkDir)
	log.Infof("Creating reset sentinel file: %s", sentinelFile)
	if err := utils.WriteFile(i.fs, api.WriteFile{
		Path: sentinelFile,
	}); err != nil {
		return fmt.Errorf("writing reset sentinel file: %w", err)
	}
	return nil
}

func (i *UnmanagedInstaller) Reset(conf api.RegistrationResponse) error {
	sentinelFile := i.formatResetSentinelFile(conf.Config.Elemental.Agent.WorkDir)
	log.Infof("Verifying reset sentinel file '%s' has been deleted", sentinelFile)
	_, err := i.fs.Stat(i.formatResetSentinelFile(conf.Config.Elemental.Agent.WorkDir))
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
