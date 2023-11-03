package hostname

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
)

type Formatter interface {
	FormatHostname(v1beta1.Hostname) (string, error)
}

func NewFormatter(osPlugin osplugin.Plugin) Formatter {
	return &formatter{
		osPlugin: osPlugin,
	}
}

var _ Formatter = (*formatter)(nil)

type formatter struct {
	osPlugin osplugin.Plugin
}

func (p *formatter) FormatHostname(conf v1beta1.Hostname) (string, error) {
	var newHostname string
	var err error
	if conf.UseExisting {
		log.Debug("Using existing hostname")
		if newHostname, err = p.formatCurrent(conf.Prefix); err != nil {
			return "", fmt.Errorf("setting current hostname: %w", err)
		}
		return newHostname, nil

	}

	log.Debug("Using random hostname")
	if newHostname, err = p.formatRandom(conf.Prefix); err != nil {
		return "", fmt.Errorf("setting random hostname: %w", err)
	}
	return newHostname, nil
}

func (p *formatter) formatRandom(prefix string) (string, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("generating new random UUID: %w", err)
	}
	return fmt.Sprintf("%s%s", prefix, uuid.String()), nil
}

func (p *formatter) formatCurrent(prefix string) (string, error) {
	currentHostname, err := p.osPlugin.GetHostname()
	if err != nil {
		return "", fmt.Errorf("getting current hostname: %w", err)
	}
	return fmt.Sprintf("%s%s", prefix, currentHostname), nil
}
