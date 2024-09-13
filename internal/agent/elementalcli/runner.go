package elementalcli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"gopkg.in/yaml.v3"
)

const (
	CorrelationIDLabelKey = "correlationID"
)

type Install struct {
	Firmware         string      `json:"firmware,omitempty" mapstructure:"firmware"`
	Device           string      `json:"device,omitempty" mapstructure:"device"`
	NoFormat         bool        `json:"noFormat,omitempty" mapstructure:"noFormat"`
	ConfigURLs       []string    `json:"configUrls,omitempty" mapstructure:"configUrls"`
	ISO              string      `json:"iso,omitempty" mapstructure:"iso"`
	SystemURI        string      `json:"systemUri,omitempty" mapstructure:"systemUri"`
	Debug            bool        `json:"debug,omitempty" mapstructure:"debug"`
	TTY              string      `json:"tty,omitempty" mapstructure:"tty"`
	EjectCD          bool        `json:"ejectCd,omitempty" mapstructure:"ejectCd"`
	DisableBootEntry bool        `json:"disableBootEntry,omitempty" mapstructure:"disableBootEntry"`
	ConfigDir        string      `json:"configDir,omitempty" mapstructure:"configDir"`
	Snapshotter      Snapshotter `json:"snapshotter,omitempty" mapstructure:"snapshotter"`
}

type Snapshotter struct {
	Type string `json:"type,omitempty" mapstructure:"type"`
}

type Reset struct {
	Enabled         bool     `json:"enabled,omitempty" mapstructure:"enabled"`
	ResetPersistent bool     `json:"resetPersistent,omitempty" mapstructure:"resetPersistent"`
	ResetOEM        bool     `json:"resetOem,omitempty" mapstructure:"resetOem"`
	ConfigURLs      []string `json:"configUrls,omitempty" mapstructure:"configUrls"`
	SystemURI       string   `json:"systemUri,omitempty" mapstructure:"systemUri"`
	Debug           bool     `json:"debug,omitempty" mapstructure:"debug"`
}

type Upgrade struct {
	ImageURI        string `json:"imageUri,omitempty" mapstructure:"imageUri"`
	UpgradeRecovery bool   `json:"upgradeRecovery,omitempty" mapstructure:"upgradeRecovery"`
	Debug           bool   `json:"debug,omitempty" mapstructure:"debug"`
}

type State struct {
	StatePartition PartitionState `yaml:"state,omitempty"`
}

type PartitionState struct {
	Snapshots map[int]*Snapshot `yaml:"snapshots,omitempty"`
}

type Snapshot struct {
	Active bool              `yaml:"active,omitempty"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

type Runner interface {
	Install(Install) error
	Reset(Reset) error
	Upgrade(Upgrade, string) error
	GetState() (State, error)
}

func NewRunner() Runner {
	return &runner{}
}

var _ Runner = (*runner)(nil)

type runner struct{}

func (r *runner) Install(conf Install) error {
	log.Debug("Running elemental install")
	installerOpts := []string{"elemental"}
	// There are no env var bindings in elemental-cli for elemental root options
	// so root flags should be passed within the command line
	if conf.Debug {
		installerOpts = append(installerOpts, "--debug")
	}

	if conf.ConfigDir != "" {
		installerOpts = append(installerOpts, "--config-dir", conf.ConfigDir)
	}
	installerOpts = append(installerOpts, "install")

	cmd := exec.Command("elemental")
	environmentVariables := mapToInstallEnv(conf)
	cmd.Env = append(os.Environ(), environmentVariables...)
	cmd.Stdout = os.Stdout
	cmd.Args = installerOpts
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	log.Debugf("running: %s\n with ENV:\n%s", strings.Join(installerOpts, " "), strings.Join(environmentVariables, "\n"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running elemental install: %w", err)
	}
	return nil
}

func (r *runner) Reset(conf Reset) error {
	log.Debug("Running elemental reset")
	installerOpts := []string{"elemental"}
	// There are no env var bindings in elemental-cli for elemental root options
	// so root flags should be passed within the command line
	if conf.Debug {
		installerOpts = append(installerOpts, "--debug")
	}
	installerOpts = append(installerOpts, "reset")

	cmd := exec.Command("elemental")
	environmentVariables := mapToResetEnv(conf)
	cmd.Env = append(os.Environ(), environmentVariables...)
	cmd.Stdout = os.Stdout
	cmd.Args = installerOpts
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	log.Debugf("running: %s\n with ENV:\n%s", strings.Join(installerOpts, " "), strings.Join(environmentVariables, "\n"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running elemental reset: %w", err)
	}
	return nil
}

func (r *runner) Upgrade(conf Upgrade, correlationID string) error {
	log.Debug("Running elemental upgrade")
	installerOpts := []string{"elemental"}
	// There are no env var bindings in elemental-cli for elemental root options
	// so root flags should be passed within the command line
	if conf.Debug {
		installerOpts = append(installerOpts, "--debug")
	}
	installerOpts = append(installerOpts, "upgrade")

	cmd := exec.Command("elemental")
	environmentVariables := mapToUpgradeEnv(conf, correlationID)
	cmd.Env = append(os.Environ(), environmentVariables...)
	cmd.Stdout = os.Stdout
	cmd.Args = installerOpts
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	log.Debugf("running: %s\n with ENV:\n%s", strings.Join(installerOpts, " "), strings.Join(environmentVariables, "\n"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running elemental upgrade: %w", err)
	}
	return nil
}

func (r *runner) GetState() (State, error) {
	state := State{}

	log.Debug("Getting elemental state")
	installerOpts := []string{"elemental", "state"}
	cmd := exec.Command("elemental")
	cmd.Args = installerOpts
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	log.Debugf("running: %s", strings.Join(installerOpts, " "))

	var commandOutput []byte
	var err error
	if commandOutput, err = cmd.Output(); err != nil {
		return state, fmt.Errorf("running elemental state: %w", err)
	}
	if err := yaml.Unmarshal(commandOutput, &state); err != nil {
		return state, fmt.Errorf("unmarshalling elemental state: %w", err)
	}

	return state, nil
}

func mapToInstallEnv(conf Install) []string {
	var variables []string
	// See GetInstallKeyEnvMap() in https://github.com/rancher/elemental-toolkit/blob/main/pkg/constants/constants.go
	variables = append(variables, formatEV("ELEMENTAL_INSTALL_CLOUD_INIT", strings.Join(conf.ConfigURLs[:], ",")))
	variables = append(variables, formatEV("ELEMENTAL_INSTALL_TARGET", conf.Device))
	variables = append(variables, formatEV("ELEMENTAL_INSTALL_SYSTEM", conf.SystemURI))
	variables = append(variables, formatEV("ELEMENTAL_INSTALL_FIRMWARE", conf.Firmware))
	variables = append(variables, formatEV("ELEMENTAL_INSTALL_ISO", conf.ISO))
	variables = append(variables, formatEV("ELEMENTAL_INSTALL_TTY", conf.TTY))
	variables = append(variables, formatEV("ELEMENTAL_INSTALL_DISABLE_BOOT_ENTRY", strconv.FormatBool(conf.DisableBootEntry)))
	variables = append(variables, formatEV("ELEMENTAL_INSTALL_NO_FORMAT", strconv.FormatBool(conf.NoFormat)))
	// See GetRunKeyEnvMap() in https://github.com/rancher/elemental-toolkit/blob/main/pkg/constants/constants.go
	variables = append(variables, formatEV("ELEMENTAL_EJECT_CD", strconv.FormatBool(conf.EjectCD)))
	variables = append(variables, formatEV("ELEMENTAL_SNAPSHOTTER_TYPE", conf.Snapshotter.Type))
	return variables
}

func mapToResetEnv(conf Reset) []string {
	var variables []string
	// See GetResetKeyEnvMap() in https://github.com/rancher/elemental-toolkit/blob/main/pkg/constants/constants.go
	variables = append(variables, formatEV("ELEMENTAL_RESET_CLOUD_INIT", strings.Join(conf.ConfigURLs[:], ",")))
	variables = append(variables, formatEV("ELEMENTAL_RESET_SYSTEM", conf.SystemURI))
	variables = append(variables, formatEV("ELEMENTAL_RESET_PERSISTENT", strconv.FormatBool(conf.ResetPersistent)))
	variables = append(variables, formatEV("ELEMENTAL_RESET_OEM", strconv.FormatBool(conf.ResetOEM)))
	return variables
}

func mapToUpgradeEnv(conf Upgrade, correlationID string) []string {
	var variables []string
	// See GetUpgradeKeyEnvMap() in https://github.com/rancher/elemental-toolkit/blob/main/pkg/constants/constants.go
	variables = append(variables, formatEV("ELEMENTAL_UPGRADE_RECOVERY", strconv.FormatBool(conf.UpgradeRecovery)))
	variables = append(variables, formatEV("ELEMENTAL_UPGRADE_SYSTEM", conf.ImageURI))
	variables = append(variables, formatEV("ELEMENTAL_UPGRADE_SNAPSHOT_LABELS", fmt.Sprintf("%s=%s", CorrelationIDLabelKey, correlationID)))
	return variables
}

func formatEV(key string, value string) string {
	return fmt.Sprintf("%s=%s", key, value)
}
