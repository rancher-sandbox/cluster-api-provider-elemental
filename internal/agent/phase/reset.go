package phase

import (
	"encoding/json"
	"fmt"
	"time"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type ResetHandler interface {
	Reset(hostname string)
	TriggerReset(hostname string) error
}

var _ ResetHandler = (*resetHandler)(nil)

func NewResetHandler(client client.Client, osPlugin osplugin.Plugin, reconciliation time.Duration) ResetHandler {
	return &resetHandler{
		client:         client,
		osPlugin:       osPlugin,
		reconciliation: reconciliation,
	}
}

type resetHandler struct {
	client         client.Client
	osPlugin       osplugin.Plugin
	reconciliation time.Duration
}

func (r *resetHandler) TriggerReset(hostname string) error {
	if err := r.osPlugin.TriggerReset(); err != nil {
		err := fmt.Errorf("triggering reset: %w", err)
		updateCondition(r.client, hostname, clusterv1.Condition{
			Type:     infrastructurev1beta1.ResetReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1beta1.ResetFailedReason,
			Message:  err.Error(),
		})
		return err
	}
	updateCondition(r.client, hostname, clusterv1.Condition{
		Type:     infrastructurev1beta1.ResetReady,
		Status:   corev1.ConditionFalse,
		Severity: infrastructurev1beta1.WaitingForResetReasonSeverity,
		Reason:   infrastructurev1beta1.WaitingForResetReason,
		Message:  "Reset was triggered successfully. Waiting for host to reset.",
	})
	return nil
}

func (r *resetHandler) Reset(hostname string) {
	r.resetLoop(hostname)
}

// installLoop **indefinitely** tries to fetch the remote registration and reset the ElementalHost
func (r *resetHandler) resetLoop(hostname string) {
	var resetError error
	alreadyReset := false
	for {
		// Wait for recovery (end user may fix the remote reset instructions meanwhile)
		if resetError != nil {
			// Log error
			log.Error(resetError, "resetting")
			// Attempt to report failed condition on management server
			updateCondition(r.client, hostname, clusterv1.Condition{
				Type:     infrastructurev1beta1.ResetReady,
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityError,
				Reason:   infrastructurev1beta1.ResetFailedReason,
				Message:  resetError.Error(),
			})
			// Clear error for next attempt
			resetError = nil
			log.Debugf("Waiting '%s' on reset error for reset instructions to mutate", r.reconciliation)
			time.Sleep(r.reconciliation)
		}
		// Mark ElementalHost for deletion
		// Repeat in case of failures. May be exploited server side to track repeated attempts.
		log.Debugf("Marking ElementalHost for deletion: %s", hostname)
		if err := r.client.DeleteHost(hostname); err != nil {
			resetError = fmt.Errorf("marking host for deletion: %w", err)
			continue
		}
		// Reset
		if !alreadyReset {
			// Fetch remote Registration
			log.Debug("Fetching remote registration")
			registration, err := r.client.GetRegistration()
			if err != nil {
				resetError = fmt.Errorf("getting remote Registration: %w", err)
				continue
			}
			log.Debug("Resetting...")
			resetBytes, err := json.Marshal(registration.Config.Elemental.Reset)
			if err != nil {
				resetError = fmt.Errorf("marshalling reset config: %w", err)
				continue
			}
			if err := r.osPlugin.Reset(resetBytes); err != nil {
				resetError = fmt.Errorf("resetting host: %w", err)
				continue
			}
			alreadyReset = true
		}
		// Report reset success
		log.Debug("Patching ElementalHost as reset")
		patchRequest := api.HostPatchRequest{Reset: ptr.To(true)}
		patchRequest.SetCondition(infrastructurev1beta1.ResetReady,
			corev1.ConditionTrue,
			clusterv1.ConditionSeverityInfo,
			"", "")
		if _, err := r.client.PatchHost(patchRequest, hostname); err != nil {
			resetError = fmt.Errorf("patching host with reset successful: %w", err)
			continue
		}
		break
	}
}
