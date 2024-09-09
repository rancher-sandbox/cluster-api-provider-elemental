package phase

import (
	"encoding/json"
	"fmt"

	infrastructurev1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type OSVersionHandler interface {
	Reconcile(map[string]runtime.RawExtension) (infrastructurev1.PostAction, error)
}

var _ OSVersionHandler = (*osVersionHandler)(nil)

func NewOSVersionHandler(agentContext context.AgentContext) OSVersionHandler {
	return &osVersionHandler{
		agentContext: agentContext,
	}
}

type osVersionHandler struct {
	agentContext context.AgentContext
}

func (o *osVersionHandler) Reconcile(osVersionManagement map[string]runtime.RawExtension) (infrastructurev1.PostAction, error) {
	post := infrastructurev1.PostAction{}
	// Serialize input to JSON
	bytes, err := json.Marshal(osVersionManagement)
	if err != nil {
		err := fmt.Errorf("marshalling Host osVersionManagement to JSON: %w", err)
		updateCondition(o.agentContext.Client, o.agentContext.Hostname, clusterv1.Condition{
			Type:     infrastructurev1.OSVersionReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1.OSVersionReconciliationFailedReason,
			Message:  err.Error(),
		})
		return post, err
	}
	// Ask the OSPlugin to reconcile
	reboot, err := o.agentContext.Plugin.ReconcileOSVersion(bytes)
	if err != nil {
		err := fmt.Errorf("reconciling osVersion: %w", err)
		updateCondition(o.agentContext.Client, o.agentContext.Hostname, clusterv1.Condition{
			Type:     infrastructurev1.OSVersionReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1.OSVersionReconciliationFailedReason,
			Message:  err.Error(),
		})
		return post, err
	}
	if reboot {
		// We only set this phase if we have to reboot, otherwise it will be most likely transitory and too spammy.
		setPhase(o.agentContext.Client, o.agentContext.Hostname, infrastructurev1.PhaseOSVersionReconcile)
		updateCondition(o.agentContext.Client, o.agentContext.Hostname, clusterv1.Condition{
			Type:     infrastructurev1.OSVersionReady,
			Status:   corev1.ConditionFalse,
			Severity: infrastructurev1.WaitingForPostReconcileRebootReasonSeverity,
			Reason:   infrastructurev1.WaitingForPostReconcileRebootReason,
			Message:  "Waiting for Host to reboot after OS Version has been reconciled.",
		})
		post.Reboot = reboot
		return post, nil
	}

	// If we are not rebooting, assume there's nothing left to do for the elemental-agent.
	if err := updateConditionOrFail(o.agentContext.Client, o.agentContext.Hostname, clusterv1.Condition{
		Type:     infrastructurev1.OSVersionReady,
		Status:   corev1.ConditionTrue,
		Severity: clusterv1.ConditionSeverityInfo,
		Reason:   "",
		Message:  "",
	}); err != nil {
		return post, fmt.Errorf("updating OSVersionReady=true condition: %w", err)
	}
	return post, nil
}
