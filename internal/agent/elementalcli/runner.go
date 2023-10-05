package elementalcli

import (
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
)

type Runner interface {
	Install(infrastructurev1beta1.Install) error
	Reset(infrastructurev1beta1.Reset) error
}

func NewRunner() Runner {
	return &runner{}
}

var _ Runner = (*runner)(nil)

type runner struct{}

func (r *runner) Install(infrastructurev1beta1.Install) error {
	log.Info("Running elemental install")
	return nil
}

func (r *runner) Reset(infrastructurev1beta1.Reset) error {
	log.Info("Running elemental reset")
	return nil
}
