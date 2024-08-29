package phase

import (
	"encoding/json"
	"fmt"
	"time"

	infrastructurev1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/context"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type ResetHandler interface {
	Reset()
	TriggerReset() error
}

var _ ResetHandler = (*resetHandler)(nil)

func NewResetHandler(agentContext context.AgentContext) ResetHandler {
	return &resetHandler{
		agentContext: agentContext,
	}
}

type resetHandler struct {
	agentContext context.AgentContext
}

func (r *resetHandler) TriggerReset() error {
	setPhase(r.agentContext.Client, r.agentContext.Hostname, infrastructurev1.PhaseTriggeringReset)
	if err := r.agentContext.Plugin.TriggerReset(); err != nil {
		err := fmt.Errorf("triggering reset: %w", err)
		updateCondition(r.agentContext.Client, r.agentContext.Hostname, clusterv1.Condition{
			Type:     infrastructurev1.ResetReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1.ResetFailedReason,
			Message:  err.Error(),
		})
		return err
	}
	updateCondition(r.agentContext.Client, r.agentContext.Hostname, clusterv1.Condition{
		Type:     infrastructurev1.ResetReady,
		Status:   corev1.ConditionFalse,
		Severity: infrastructurev1.WaitingForResetReasonSeverity,
		Reason:   infrastructurev1.WaitingForResetReason,
		Message:  "Reset was triggered successfully. Waiting for host to reset.",
	})
	return nil
}

func (r *resetHandler) Reset() {
	setPhase(r.agentContext.Client, r.agentContext.Hostname, infrastructurev1.PhaseResetting)
	r.resetLoop()
}

// installLoop **indefinitely** tries to fetch the remote registration and reset the ElementalHost.
func (r *resetHandler) resetLoop() {
	var resetError error
	alreadyReset := false
	for {
		// Wait for recovery (end user may fix the remote reset instructions meanwhile)
		if resetError != nil {
			// Log error
			log.Error(resetError, "resetting")
			// Attempt to report failed condition on management server
			updateCondition(r.agentContext.Client, r.agentContext.Hostname, clusterv1.Condition{
				Type:     infrastructurev1.ResetReady,
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityError,
				Reason:   infrastructurev1.ResetFailedReason,
				Message:  resetError.Error(),
			})
			// Clear error for next attempt
			resetError = nil
			log.Debugf("Waiting '%s' on reset error for reset instructions to mutate", r.agentContext.Config.Agent.Reconciliation)
			time.Sleep(r.agentContext.Config.Agent.Reconciliation)
		}
		// Mark ElementalHost for deletion
		// Repeat in case of failures. May be exploited server side to track repeated attempts.
		log.Debugf("Marking ElementalHost for deletion: %s", r.agentContext.Hostname)
		if err := r.agentContext.Client.DeleteHost(r.agentContext.Hostname); err != nil {
			resetError = fmt.Errorf("marking host for deletion: %w", err)
			continue
		}
		// Reset
		if !alreadyReset {
			// Fetch remote Registration
			log.Debug("Fetching remote registration")
			registration, err := r.agentContext.Client.GetRegistration()
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
			if err := r.agentContext.Plugin.Reset(resetBytes); err != nil {
				resetError = fmt.Errorf("resetting host: %w", err)
				continue
			}
			alreadyReset = true
		}
		// Report reset success
		log.Debug("Patching ElementalHost as reset")
		patchRequest := api.HostPatchRequest{Reset: ptr.To(true)}
		patchRequest.SetCondition(infrastructurev1.ResetReady,
			corev1.ConditionTrue,
			clusterv1.ConditionSeverityInfo,
			"", "")
		if _, err := r.agentContext.Client.PatchHost(patchRequest, r.agentContext.Hostname); err != nil {
			resetError = fmt.Errorf("patching host with reset successful: %w", err)
			continue
		}
		break
	}
}
