package phase

import (
	"errors"
	"fmt"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/phase/phases"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	"github.com/twpayne/go-vfs/v4"
)

var ErrUknownPhase = errors.New("Can not handle unknown phase")

type HostContext struct {
	AgentConfig     config.Config
	AgentConfigPath string
	Hostname        string
}

type HostPhaseHandler interface {
	Init(fs vfs.FS, client client.Client, osPlugin osplugin.Plugin, id identity.Identity, hostContext HostContext)
	Handle(infrastructurev1beta1.HostPhase) (phases.PostAction, error)
}

var _ HostPhaseHandler = (*hostPhaseHandler)(nil)

func NewHostPhaseHandler() HostPhaseHandler {
	return &hostPhaseHandler{}
}

func (h *hostPhaseHandler) Init(fs vfs.FS, client client.Client, osPlugin osplugin.Plugin, id identity.Identity, hostContext HostContext) {
	h.client = client

	h.hostContext = hostContext

	h.register = phases.NewRegistrationHandler(client, osPlugin, id, hostContext.AgentConfig.Agent.Reconciliation)
	h.install = phases.NewInstallHandler(client, osPlugin, id, hostContext.AgentConfig.Agent.Reconciliation)
	h.bootstrap = phases.NewBootstrapHandler(fs, client, osPlugin)
	h.reset = phases.NewResetHandler(client, osPlugin, hostContext.AgentConfig.Agent.Reconciliation)
}

type hostPhaseHandler struct {
	client client.Client

	register  phases.RegistrationHandler
	install   phases.InstallHandler
	bootstrap phases.BootstrapHandler
	reset     phases.ResetHandler

	hostContext HostContext
}

func (h *hostPhaseHandler) Handle(phase infrastructurev1beta1.HostPhase) (phases.PostAction, error) {
	switch phase {
	case infrastructurev1beta1.PhaseRegistering:
		hostname, agentConfig, err := h.register.Register()
		if err != nil {
			return phases.PostAction{}, fmt.Errorf("registering new host: %w", err)
		}
		h.hostContext.Hostname = hostname
		h.hostContext.AgentConfig = agentConfig
		h.setPhase(phase) // Note that we set the phase **after* its conclusion, because we do not have any remote ElementalHost to patch before.
	case infrastructurev1beta1.PhaseFinalizingRegistration:
		h.setPhase(phase)
		if err := h.register.FinalizeRegistration(h.hostContext.Hostname, h.hostContext.AgentConfigPath, h.hostContext.AgentConfig); err != nil {
			return phases.PostAction{}, fmt.Errorf("finalizing registration: %w", err)
		}
	case infrastructurev1beta1.PhaseInstalling:
		h.setPhase(phase)
		h.install.Install(h.hostContext.Hostname)
		return phases.PostAction{
			Reboot:   h.hostContext.AgentConfig.Agent.PostInstall.Reboot,
			PowerOff: h.hostContext.AgentConfig.Agent.PostInstall.PowerOff,
		}, nil
	case infrastructurev1beta1.PhaseBootstrapping:
		h.setPhase(phase)
		post, err := h.bootstrap.Bootstrap(h.hostContext.Hostname)
		if err != nil {
			return phases.PostAction{}, fmt.Errorf("bootstrapping host: %w", err)
		}
		return post, nil
	case infrastructurev1beta1.PhaseRunning:
		h.setPhase(phase)
		// TODO: Implement a Running phase. For example to reconcile host information, statuses, labels, etc.
	case infrastructurev1beta1.PhaseTriggeringReset:
		h.setPhase(phase)
		if err := h.reset.TriggerReset(h.hostContext.Hostname); err != nil {
			return phases.PostAction{}, fmt.Errorf("triggering reset: %w", err)
		}
		return phases.PostAction{}, nil
	case infrastructurev1beta1.PhaseResetting:
		h.setPhase(phase)
		h.reset.Reset(h.hostContext.Hostname)
		return phases.PostAction{
			Reboot:   h.hostContext.AgentConfig.Agent.PostReset.Reboot,
			PowerOff: h.hostContext.AgentConfig.Agent.PostReset.PowerOff,
		}, nil
	default:
		return phases.PostAction{}, fmt.Errorf("handling '%s' phase: %w", phase, ErrUknownPhase)
	}
	return phases.PostAction{}, nil
}

// setPhase is a best-effort attempt to reconcile the remote HostPhase.
// In case of failures (ex. due to connection errors), it should eventually recover.
func (h *hostPhaseHandler) setPhase(phase infrastructurev1beta1.HostPhase) {
	if _, err := h.client.PatchHost(api.HostPatchRequest{
		Phase: &phase,
	}, h.hostContext.Hostname); err != nil {
		log.Errorf(err, "reporting phase: %s", phase)
	}
}
