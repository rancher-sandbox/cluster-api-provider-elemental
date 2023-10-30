package installer

import (
	"fmt"
	"os"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/twpayne/go-vfs"
)

var _ Installer = (*UnmanagedInstaller)(nil)

type UnmanagedInstaller struct {
	fs          vfs.FS
	hostManager host.Manager
	configPath  string
	workDir     string
}

func NewUnmanagedInstaller(fs vfs.FS, hostManager host.Manager, configPath string, workDir string) Installer {
	return &UnmanagedInstaller{
		fs:          fs,
		hostManager: hostManager,
		configPath:  configPath,
		workDir:     workDir,
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
	if err := i.hostManager.SetHostname(hostnameToSet); err != nil {
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
