package config

import (
	"time"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
)

// Defaults.
const (
	defaultWorkDir        = "/var/lib/elemental/agent"
	defaultReconciliation = 10 * time.Second
	defaultPlugin         = "/usr/lib/elemental/plugins/elemental.so"
)

// Config represents the CAPI Elemental agent configuration.
type Config struct {
	// Registration config
	Registration infrastructurev1beta1.Registration `yaml:"registration" mapstructure:"registration"`
	// Agent config
	Agent infrastructurev1beta1.Agent `yaml:"agent" mapstructure:"agent"`
}

// FromInfrastructure can be used to convert the ElementalRegistration CAPI infrastructure resource to an agent config file.
// This function can be used by the operator to generate an initial agent config.
func FromInfrastructure(conf infrastructurev1beta1.Config) Config {
	return Config{
		Registration: conf.Elemental.Registration,
		Agent:        conf.Elemental.Agent,
	}
}

// FromAPI can be used to convert the Elemental API Registration resource to an agent config file.
// This function can be used by the client to update the local config to match the remote configuration.
func FromAPI(conf api.RegistrationResponse) Config {
	return Config{
		Registration: conf.Config.Elemental.Registration,
		Agent:        conf.Config.Elemental.Agent,
	}
}

func DefaultConfig() Config {
	return Config{
		Agent: infrastructurev1beta1.Agent{
			WorkDir:        defaultWorkDir,
			Reconciliation: defaultReconciliation,
			OSPlugin:       defaultPlugin,
		},
	}
}
