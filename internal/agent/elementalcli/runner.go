package elementalcli

import (
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
)

type Install struct {
	Firmware         string   `json:"firmware,omitempty"`
	Device           string   `json:"device,omitempty"`
	NoFormat         bool     `json:"noFormat,omitempty"`
	ConfigURLs       []string `json:"configUrls,omitempty"`
	ISO              string   `json:"iso,omitempty"`
	SystemURI        string   `json:"systemUri,omitempty"`
	Debug            bool     `json:"debug,omitempty"`
	TTY              string   `json:"tty,omitempty"`
	PowerOff         bool     `json:"poweroff,omitempty"`
	Reboot           bool     `json:"reboot,omitempty"`
	EjectCD          bool     `json:"ejectCd,omitempty"`
	DisableBootEntry bool     `json:"disableBootEntry,omitempty"`
	ConfigDir        string   `json:"configDir,omitempty"`
}

type Reset struct {
	Enabled         bool     `json:"enabled,omitempty"`
	ResetPersistent bool     `json:"resetPersistent,omitempty"`
	ResetOEM        bool     `json:"resetOem,omitempty"`
	ConfigURLs      []string `json:"configUrls,omitempty"`
	SystemURI       string   `json:"systemUri,omitempty"`
	Debug           bool     `json:"debug,omitempty"`
	PowerOff        bool     `json:"poweroff,omitempty"`
	Reboot          bool     `json:"reboot,omitempty"`
}

type Runner interface {
	Install(Install) error
	Reset(Reset) error
}

func NewRunner() Runner {
	return &runner{}
}

var _ Runner = (*runner)(nil)

type runner struct{}

func (r *runner) Install(Install) error {
	log.Debug("Running elemental install")
	return nil
}

func (r *runner) Reset(Reset) error {
	log.Debug("Running elemental reset")
	return nil
}
