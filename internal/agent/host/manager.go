package host

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
)

type Manager interface {
	SetHostname(hostname string) error
	PickHostname(conf infrastructurev1beta1.Hostname) (string, error)
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

func (m *manager) SetHostname(hostname string) error {
	log.Debugf("Setting hostname: %s", hostname)
	if err := m.cmdRunner.RunCommand(fmt.Sprintf("hostnamectl set-hostname %s", hostname)); err != nil {
		return fmt.Errorf("running hostnamectl: %w", err)
	}
	return nil
}

func (m *manager) PickHostname(conf infrastructurev1beta1.Hostname) (string, error) {
	var newHostname string
	var err error
	if conf.UseExisting {
		log.Debug("Using existing hostname")
		if newHostname, err = m.formatCurrent(conf.Prefix); err != nil {
			return "", fmt.Errorf("setting current hostname: %w", err)
		}
		return newHostname, nil

	}

	log.Debug("Using random hostname")
	if newHostname, err = m.formatRandom(conf.Prefix); err != nil {
		return "", fmt.Errorf("setting random hostname: %w", err)
	}
	return newHostname, nil
}

func (m *manager) GetCurrentHostname() (string, error) {
	currentHostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("getting current hostname: %w", err)
	}
	return currentHostname, nil
}

func (m *manager) formatRandom(prefix string) (string, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("generating new random UUID: %w", err)
	}
	return fmt.Sprintf("%s%s", prefix, uuid.String()), nil
}

func (m *manager) formatCurrent(prefix string) (string, error) {
	currentHostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("getting current hostname: %w", err)
	}
	return fmt.Sprintf("%s%s", prefix, currentHostname), nil
}
