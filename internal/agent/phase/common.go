package phase

import (
	"fmt"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// updateCondition is a best effort method to update the remote condition.
// Due to the unexpected nature of failures, we should not attempt indefinitely as there is no indication for recovery.
// For example if a network error occurs, leading to a failed condition, it's likely that reporting the condition will fail as well.
// The controller should always try to reconcile the 'True' status for each Host condition, so reporting failures should not be critical.
func updateCondition(client client.Client, hostname string, condition clusterv1.Condition) {
	if _, err := client.PatchHost(api.HostPatchRequest{
		Condition: &condition,
	}, hostname); err != nil {
		log.Error(err, "reporting condition", "conditionType", condition.Type, "conditionReason", condition.Reason)
	}
}

// updateConditionOrFail should be used to set 'Status: True' conditions.
// In case of errors we should re-attempt, otherwise a condition may never be set to True.
func updateConditionOrFail(client client.Client, hostname string, condition clusterv1.Condition) error {
	if _, err := client.PatchHost(api.HostPatchRequest{
		Condition: &condition,
	}, hostname); err != nil {
		return fmt.Errorf("reporting condition: %w", err)
	}
	return nil
}
