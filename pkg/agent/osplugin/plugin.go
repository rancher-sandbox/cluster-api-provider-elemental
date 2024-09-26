package osplugin

import (
	"fmt"
	"plugin"
)

const (
	// GetPluginSymbol is the symbol expected to return a Plugin implementation.
	GetPluginSymbol = "GetPlugin"
)

// PluginContext contains information to be passed to any plugin.
type PluginContext struct {
	// WorkDir is the agent work directory
	WorkDir string
	// ConfigPath is the agent full config path
	ConfigPath string
	// Debug options should be enabled
	Debug bool
}

// Plugin represents the OS Plugin interface.
// Any Plugin is expected to fully implement the interface.
type Plugin interface {
	// Init is called just after the plugin is loaded to pass context information.
	Init(PluginContext) error
	// GetHostname should return the current machine hostname.
	GetHostname() (string, error)
	// InstallCloudInit should install a cloud-init input config (in JSON format) to the machine.
	InstallCloudInit(input []byte) error
	// InstallHostname should install the input hostname to the machine.
	InstallHostname(hostname string) error
	// InstallFile should install any file to the input path, given a content.
	InstallFile(content []byte, path string, permission uint32, owner int, group int) error
	// Install should install any needed components to the machine, given an input install config (in JSON format).
	// This is called by the agent on 'install' command.
	Install(input []byte) error
	// Bootstrap should apply the CAPI bootstrap config to the machine.
	// The format can be either "cloud-init" or "ignition".
	Bootstrap(format string, input []byte) error
	// ReconcileOSVersion should reconcile the OS version on the host according to the input (in JSON format).
	// You can trigger a Reboot by returning a true value. Note that in case of error this is ignored.
	ReconcileOSVersion(input []byte) (bool, error)
	// TriggerReset should prepare the machine for reset.
	TriggerReset() error
	// Reset should reset the machine to an installable state, given an input reset config (in JSON format).
	// This is called by the agent on 'reset' command.
	Reset(input []byte) error
	// PowerOff should poweroff the machine.
	PowerOff() error
	// Reboot should reboot the machine.
	Reboot() error
}

// Loader is a simple plugin loader.
type Loader interface {
	Load(string) (Plugin, error)
}

// NewLoader returns a simple Loader implementation.
func NewLoader() Loader {
	return &loader{}
}

var _ Loader = (*loader)(nil)

type loader struct{}

// Load loads a plugin given an input path.
// If the plugin can not be loaded or the interface is not implemented by the plugin, an error is returned.
func (l *loader) Load(path string) (Plugin, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening plugin in path '%s': %w", path, err)
	}

	getPlugin, err := p.Lookup(GetPluginSymbol)
	if err != nil {
		return nil, fmt.Errorf("looking up symbol '%s': %w", GetPluginSymbol, err)
	}

	plugin, err := getPlugin.(func() (Plugin, error))()
	if err != nil {
		return nil, fmt.Errorf("getting plugin: %w", err)
	}

	return plugin, nil
}
