package hostname

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
)

func SetHostname(hostname string) error {
	if err := utils.RunCommand(fmt.Sprintf("hostnamectl set-hostname %s", hostname)); err != nil {
		return fmt.Errorf("running hostnamectl: %w", err)
	}
	return nil
}

func PickHostname(conf infrastructurev1beta1.Hostname) (string, error) {
	var newHostname string
	var err error
	if conf.UseExisting {
		if newHostname, err = formatCurrent(conf.Prefix); err != nil {
			return "", fmt.Errorf("setting current hostname: %w", err)
		}
		return newHostname, nil

	}

	if newHostname, err = formatRandom(conf.Prefix); err != nil {
		return "", fmt.Errorf("setting random hostname: %w", err)
	}
	return newHostname, nil
}

func GetCurrentHostname() (string, error) {
	currentHostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("getting current hostname: %w", err)
	}
	return currentHostname, nil
}

func formatRandom(prefix string) (string, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("generating new random UUID: %w", err)
	}
	return fmt.Sprintf("%s%s", prefix, uuid.String()), nil
}

func formatCurrent(prefix string) (string, error) {
	currentHostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("getting current hostname: %w", err)
	}
	return fmt.Sprintf("%s%s", prefix, currentHostname), nil
}
