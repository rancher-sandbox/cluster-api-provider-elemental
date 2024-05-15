package phases

import (
	"fmt"
	"os"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	"github.com/twpayne/go-vfs/v4"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
)

const (
	bootstrapSentinelFile = "/run/cluster-api/bootstrap-success.complete"
)

type BootstrapHandler interface {
	Bootstrap(hostname string) (PostAction, error)
}

var _ BootstrapHandler = (*bootstrapHandler)(nil)

func NewBootstrapHandler(fs vfs.FS, client client.Client, osPlugin osplugin.Plugin) BootstrapHandler {
	return &bootstrapHandler{
		fs:       fs,
		client:   client,
		osPlugin: osPlugin,
	}
}

type bootstrapHandler struct {
	fs       vfs.FS
	client   client.Client
	osPlugin osplugin.Plugin
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
func (b *bootstrapHandler) Bootstrap(hostname string) (PostAction, error) {
	post, err := b.bootstrap(hostname)
	if err != nil {
		updateCondition(b.client, hostname, clusterv1.Condition{
			Type:     infrastructurev1beta1.BootstrapReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1beta1.BootstrapFailedReason,
			Message:  err.Error(),
		})
	}
	return post, err
}

func (b *bootstrapHandler) bootstrap(hostname string) (PostAction, error) {
	_, err := b.fs.Stat(bootstrapSentinelFile)

	// Assume system is successfully bootstrapped if sentinel file is found
	if err == nil {
		log.Infof("Found file: %s. System is bootstrapped.", bootstrapSentinelFile)
		if err := b.updateBoostrappedStatus(hostname); err != nil {
			return PostAction{}, fmt.Errorf("updating bootstrapped status: %w", err)
		}
		log.Info("Bootstrap config applied successfully")
		return PostAction{}, nil
	}

	// Sentinel file not found, assume system needs bootstrapping
	if os.IsNotExist(err) {
		log.Debug("Fetching bootstrap config")
		bootstrap, err := b.client.GetBootstrap(hostname)
		if err != nil {
			return PostAction{}, fmt.Errorf("fetching bootstrap config: %w", err)
		}
		log.Info("Applying bootstrap config")
		if err := b.osPlugin.Bootstrap(bootstrap.Format, []byte(bootstrap.Config)); err != nil {
			return PostAction{}, fmt.Errorf("applying bootstrap config: %w", err)
		}
		updateCondition(b.client, hostname, clusterv1.Condition{
			Type:     infrastructurev1beta1.BootstrapReady,
			Status:   corev1.ConditionFalse,
			Severity: infrastructurev1beta1.WaitingForBootstrapReasonSeverity,
			Reason:   infrastructurev1beta1.WaitingForBootstrapReason,
			Message:  "Waiting for bootstrap to be executed",
		})
		log.Info("System is rebooting to execute the bootstrap configuration...")
		return PostAction{Reboot: true}, nil
	}

	return PostAction{}, fmt.Errorf("reading file '%s': %w", bootstrapSentinelFile, err)
}

func (b *bootstrapHandler) updateBoostrappedStatus(hostname string) error {
	patchRequest := api.HostPatchRequest{Bootstrapped: ptr.To(true)}
	patchRequest.SetCondition(infrastructurev1beta1.BootstrapReady,
		corev1.ConditionTrue,
		clusterv1.ConditionSeverityInfo,
		"", "")
	if _, err := b.client.PatchHost(patchRequest, hostname); err != nil {
		return fmt.Errorf("patching bootstrapped status: %w", err)
	}
	return nil
}
