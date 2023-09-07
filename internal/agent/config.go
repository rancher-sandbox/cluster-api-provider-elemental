package agent

import "time"

const (
	DefaultReconciliation = 1 * time.Minute
)

type Config struct {
	Registration Registration `yaml:"registration" mapstructure:"registration"`
	Agent        Agent        `yaml:"agent" mapstructure:"agent"`
}

type Registration struct {
	URI    string `yaml:"uri,omitempty" mapstructure:"uri"`
	CACert string `yaml:"caCert,omitempty" mapstructure:"caCert"`
}

type Agent struct {
	Debug              bool          `yaml:"debug,omitempty" mapstructure:"debug"`
	Reconciliation     time.Duration `yaml:"reconciliation,omitempty" mapstructure:"reconciliation"`
	InsecureSkipVerify bool          `yaml:"insecureSkipVerify,omitempty" mapstructure:"insecureSkipVerify"`
	UseSystemCertPool  bool          `yaml:"useSystemCertPool,omitempty" mapstructure:"useSystemCertPool"`
}

func DefaultConfig() Config {
	return Config{
		Agent: Agent{
			Reconciliation:    DefaultReconciliation,
			UseSystemCertPool: true,
		},
	}
}
