package installer

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
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
		installer = NewUnmanagedInstaller(fs, host.NewManager(), configPath, conf.Agent.WorkDir)
	case installerElemental:
		log.Info("Using Elemental Installer")
		installer = NewElementalInstaller(fs, host.NewManager(), configPath, conf.Agent.WorkDir)
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
