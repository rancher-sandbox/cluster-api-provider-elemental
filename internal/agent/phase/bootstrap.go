package phase

import (
	"fmt"
	"os"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/context"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/twpayne/go-vfs/v4"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrastructurev1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
)

const (
	bootstrapSentinelFile = "/run/cluster-api/bootstrap-success.complete"
)

type BootstrapHandler interface {
	Bootstrap() (infrastructurev1.PostAction, error)
}

var _ BootstrapHandler = (*bootstrapHandler)(nil)

func NewBootstrapHandler(agentContext context.AgentContext) BootstrapHandler {
	return &bootstrapHandler{
		agentContext: agentContext,
		fs:           vfs.OSFS,
	}
}

type bootstrapHandler struct {
	agentContext context.AgentContext
	fs           vfs.FS
}

// Bootstrap is usually called twice during the bootstrap phase.
//
// The first call should normally fetch the remote bootstrap config and propagate it to the plugin implementation.
// The system should then reboot, and upon successful reboot, the `/run/cluster-api/bootstrap-success.complete`
// sentinel file is expected to exist.
// Note that the reboot is currently enforced, since both `cloud-init` and `ignition` formats are meant to be applied
// during system boot.
// See: https://cluster-api.sigs.k8s.io/developer/providers/bootstrap.html#sentinel-file
//
// The second call should normally patch the remote Host resource as bootstrapped,
// after verifying the existance of `/run/cluster-api/bootstrap-success.complete`.
// Note that since `/run` is normally mounted as tmpfs and the bootstrap config is not re-executed at every boot,
// the remote host needs to be patched before the system is ever rebooted an additional time.
// If reboot happens and `/run/cluster-api/bootstrap-success.complete` is not found on the already-bootstrapped system,
// the plugin will be invoked again to re-apply the bootstrap config. It's up to the plugin implementation to recover
// from this state if possible, or to just return an error to highlight manual intervention is needed (and possibly a machine reset).
func (b *bootstrapHandler) Bootstrap() (infrastructurev1.PostAction, error) {
	setPhase(b.agentContext.Client, b.agentContext.Hostname, infrastructurev1.PhaseBootstrapping)
	post, err := b.bootstrap()
	if err != nil {
		updateCondition(b.agentContext.Client, b.agentContext.Hostname, clusterv1.Condition{
			Type:     infrastructurev1.BootstrapReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1.BootstrapFailedReason,
			Message:  err.Error(),
		})
	}
	return post, err
}

func (b *bootstrapHandler) bootstrap() (infrastructurev1.PostAction, error) {
	_, err := b.fs.Stat(bootstrapSentinelFile)

	// Assume system is successfully bootstrapped if sentinel file is found
	if err == nil {
		log.Infof("Found file: %s. System is bootstrapped.", bootstrapSentinelFile)
		if err := b.updateBoostrappedStatus(b.agentContext.Hostname); err != nil {
			return infrastructurev1.PostAction{}, fmt.Errorf("updating bootstrapped status: %w", err)
		}
		log.Info("Bootstrap config applied successfully")
		return infrastructurev1.PostAction{}, nil
	}

	// Sentinel file not found, assume system needs bootstrapping
	if os.IsNotExist(err) {
		log.Debug("Fetching bootstrap config")
		bootstrap, err := b.agentContext.Client.GetBootstrap(b.agentContext.Hostname)
		if err != nil {
			return infrastructurev1.PostAction{}, fmt.Errorf("fetching bootstrap config: %w", err)
		}
		log.Info("Applying bootstrap config")
		if err := b.agentContext.Plugin.Bootstrap(bootstrap.Format, []byte(bootstrap.Config)); err != nil {
			return infrastructurev1.PostAction{}, fmt.Errorf("applying bootstrap config: %w", err)
		}
		updateCondition(b.agentContext.Client, b.agentContext.Hostname, clusterv1.Condition{
			Type:     infrastructurev1.BootstrapReady,
			Status:   corev1.ConditionFalse,
			Severity: infrastructurev1.WaitingForBootstrapReasonSeverity,
			Reason:   infrastructurev1.WaitingForBootstrapReason,
			Message:  "Waiting for bootstrap to be executed",
		})
		log.Info("System is rebooting to execute the bootstrap configuration...")
		return infrastructurev1.PostAction{Reboot: true}, nil
	}

	return infrastructurev1.PostAction{}, fmt.Errorf("reading file '%s': %w", bootstrapSentinelFile, err)
}

func (b *bootstrapHandler) updateBoostrappedStatus(hostname string) error {
	patchRequest := api.HostPatchRequest{Bootstrapped: ptr.To(true)}
	patchRequest.SetCondition(infrastructurev1.BootstrapReady,
		corev1.ConditionTrue,
		clusterv1.ConditionSeverityInfo,
		"", "")
	if _, err := b.agentContext.Client.PatchHost(patchRequest, hostname); err != nil {
		return fmt.Errorf("patching bootstrapped status: %w", err)
	}
	return nil
}
