package host

import (
	"errors"
	"fmt"

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

var ErrManagedOSNotSupportedYet = errors.New("managed Elemental OS not supported yet")

type Installer interface {
	Install(conf api.RegistrationResponse, hostnameToSet string) error
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

func (i *UnmanagedInstaller) Reset(conf api.RegistrationResponse) error {
	log.Debugf("Will not reset unmanaged OS. Creating reset sentinel file: %s/%s", conf.Config.Elemental.Agent.WorkDir, sentinelFileResetNeeded)
	if err := utils.WriteFile(i.fs, api.WriteFile{
		Path: fmt.Sprintf("%s/%s", conf.Config.Elemental.Agent.WorkDir, sentinelFileResetNeeded),
	}); err != nil {
		return fmt.Errorf("writing reset sentinel file: %w", err)
	}

	log.Debug("Resetting hostname")
	if err := i.resetHostname(); err != nil {
		return fmt.Errorf("resetting hostname: %w", err)
	}

	return nil
}

func (i *UnmanagedInstaller) resetHostname() error {
	log.Debug("Deleting '/etc/hostname'")
	if err := i.fs.Remove(hostnameFile); err != nil {
		return fmt.Errorf("deleting '%s': %w", hostnameFile, err)
	}
	return nil
}
