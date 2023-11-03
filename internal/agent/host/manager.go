package host

import (
	"fmt"
	"os"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
)

type Manager interface {
	PowerOff() error
	Reboot() error
	SetHostname(hostname string) error
	GetCurrentHostname() (string, error)
}

func NewManager() Manager {
	return &manager{
		cmdRunner: utils.NewCommandRunner(),
	}
}

var _ Manager = (*manager)(nil)

type manager struct {
	cmdRunner utils.CommandRunner
}

func (m *manager) PowerOff() error {
	if err := m.cmdRunner.RunCommand("poweroff -f"); err != nil {
		return fmt.Errorf("calling 'poweroff -f': %w", err)
	}
	return nil
}

func (m *manager) Reboot() error {
	if err := m.cmdRunner.RunCommand("reboot -f"); err != nil {
		return fmt.Errorf("calling 'reboot -f'': %w", err)
	}
	return nil
}

func (m *manager) SetHostname(hostname string) error {
	log.Debugf("Setting hostname: %s", hostname)
	if err := m.cmdRunner.RunCommand(fmt.Sprintf("hostnamectl set-hostname %s", hostname)); err != nil {
		return fmt.Errorf("running hostnamectl: %w", err)
	}
	return nil
}

func (m *manager) GetCurrentHostname() (string, error) {
	currentHostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("getting current hostname: %w", err)
	}
	return currentHostname, nil
}
