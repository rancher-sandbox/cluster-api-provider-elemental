/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/controller/utils"
	ilog "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/log"
)

var (
	ErrMissingHostReference = errors.New("missing host reference")
)

// ElementalMachineReconciler reconciles a ElementalMachine object.
type ElementalMachineReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Tracker       utils.RemoteTracker
	RequeuePeriod time.Duration
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElementalMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta1.ElementalMachine{}).
		Watches(
			&infrastructurev1beta1.ElementalHost{},
			handler.EnqueueRequestsFromMapFunc(r.ElementalHostToElementalMachine),
		).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(r.MachineToElementalMachine),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.ClusterToElementalMachines),
			// Reconciliation step #5: If the associated Cluster‘s status.infrastructureReady is false, exit the reconciliation
			// Note: This check should not be blocking any further delete reconciliation flows.
			// Note: This check should only be performed after appropriate owner references (if any) are updated.
			builder.WithPredicates(predicates.ClusterUnpausedAndInfrastructureReady(ctrl.LoggerFrom(ctx))),
		).
		Complete(r); err != nil {
		return fmt.Errorf("initializing ElementalMachineReconciler builder: %w", err)
	}
	return nil
}

func (r *ElementalMachineReconciler) ElementalHostToElementalMachine(ctx context.Context, obj client.Object) []ctrl.Request {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, obj.GetNamespace()).
		WithValues(ilog.KeyElementalHost, obj.GetName())
	logger.Info("Enqueueing ElementalMachine reconciliation from ElementalHost")

	requests := []ctrl.Request{}

	// Verify we are actually handling a ElementalHost object
	host, ok := obj.(*infrastructurev1beta1.ElementalHost)
	if !ok {
		logger.Error(ErrEnqueueing, fmt.Sprintf("Expected a ElementalHost object, but got %T", obj))
		return []ctrl.Request{}
	}

	// Check the ElementalHost was associated to any ElementalMachine
	if host.Spec.MachineRef != nil {
		logger.Info("Adding ElementalMachine to reconciliation request", ilog.KeyElementalMachine, host.Spec.MachineRef.Name)
		name := client.ObjectKey{Namespace: host.Spec.MachineRef.Namespace, Name: host.Spec.MachineRef.Name}
		requests = append(requests, ctrl.Request{NamespacedName: name})
	}

	return requests
}

func (r *ElementalMachineReconciler) MachineToElementalMachine(ctx context.Context, obj client.Object) []ctrl.Request {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, obj.GetNamespace()).
		WithValues(ilog.KeyMachine, obj.GetName())
	logger.Info("Enqueueing ElementalMachine reconciliation from Machine")

	requests := []ctrl.Request{}
	// Verify we are actually handling a Machine object
	machine, ok := obj.(*clusterv1.Machine)
	if !ok {
		logger.Error(ErrEnqueueing, fmt.Sprintf("Expected a Machine object, but got %T", obj))
		return []ctrl.Request{}
	}

	// Check the Machine was associated to any ElementalMachine
	if machine.Spec.InfrastructureRef.Kind == "ElementalMachine" {
		logger.Info("Adding ElementalMachine to reconciliation request", ilog.KeyElementalMachine, machine.Spec.InfrastructureRef.Name)
		name := client.ObjectKey{Namespace: machine.Spec.InfrastructureRef.Namespace, Name: machine.Spec.InfrastructureRef.Name}
		requests = append(requests, ctrl.Request{NamespacedName: name})
	}

	return requests
}

// ClusterToElementalMachines is a handler.ToRequestsFunc to be used to enqeue requests for reconciliation of ElementalMachines.
func (r *ElementalMachineReconciler) ClusterToElementalMachines(ctx context.Context, obj client.Object) []ctrl.Request {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, obj.GetNamespace()).
		WithValues(ilog.KeyCluster, obj.GetName())

	logger.Info("Enqueueing ElementalMachines reconciliation from Cluster")

	requests := []ctrl.Request{}

	// Verify we are actually handling a Cluster object
	cluster, ok := obj.(*clusterv1.Cluster)
	if !ok {
		logger.Error(ErrEnqueueing, fmt.Sprintf("Expected a Cluster object, but got %T", obj))
		return []ctrl.Request{}
	}

	// Fetch the MachineList associated to this Cluster
	labels := map[string]string{clusterv1.ClusterNameLabel: cluster.Name}
	capiMachineList := &clusterv1.MachineList{}
	if err := r.Client.List(ctx, capiMachineList, client.InNamespace(cluster.Namespace),
		client.MatchingLabels(labels),
	); err != nil {
		logger.Error(err, "failed to list ElementalMachines")
		return []ctrl.Request{}
	}

	// Enqueue related (same NamespacedName of Machines) ElementalMachines for reconciliation
	for i, m := range capiMachineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
		if m.Spec.InfrastructureRef.Namespace != "" {
			name = client.ObjectKey{Namespace: m.Spec.InfrastructureRef.Namespace, Name: m.Spec.InfrastructureRef.Name}
		}
		logger.Info("Adding ElementalMachine to reconciliation request", ilog.KeyElementalMachine, capiMachineList.Items[i].Name)
		requests = append(requests, ctrl.Request{NamespacedName: name})
	}

	return requests
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalmachines/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;machines,verbs=get;list;watch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
// For more details about the reconciliation loop, check the official CAPI documentation:
// - https://cluster-api.sigs.k8s.io/developer/providers/machine-infrastructure
func (r *ElementalMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, req.Namespace).
		WithValues(ilog.KeyElementalMachine, req.Name)
	logger.Info("Reconciling ElementalMachine")

	// Fetch the ElementalMachine
	elementalMachine := &infrastructurev1beta1.ElementalMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, elementalMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching ElementalMachine: %w", err)
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(elementalMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("initializing patch helper: %w", err)
	}
	defer func() {
		// Reconcile Summary Condition
		conditions.SetSummary(elementalMachine)
		// Reconciliation step #12: Patch the resource to persist changes
		if err := patchHelper.Patch(ctx, elementalMachine); err != nil {
			rerr = errors.Join(rerr, fmt.Errorf("patching ElementalMachine: %w", err))
		}
	}()

	// Reconciliation step #1: If the resource does not have a Machine owner, exit the reconciliation
	machine, err := util.GetOwnerMachine(ctx, r.Client, elementalMachine.ObjectMeta)
	if err != nil {
		err := fmt.Errorf("getting Machine owner: %w", err)
		conditions.Set(elementalMachine, &clusterv1.Condition{
			Type:     infrastructurev1beta1.AssociationReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1beta1.MissingMachineOwnerReason,
			Message:  err.Error(),
		})
		return ctrl.Result{}, err
	}
	if machine == nil {
		logger.Info("ElementalMachine resource has no Machine owner")
		conditions.Set(elementalMachine, &clusterv1.Condition{
			Type:     infrastructurev1beta1.AssociationReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1beta1.MissingMachineOwnerReason,
			Message:  "ElementalMachine resource has no Machine owner",
		})
		return ctrl.Result{}, nil
	}

	// Reconciliation step #2: If the resource has status.failureReason or status.failureMessage set, exit the reconciliation
	// TODO: status.failureReason and failureMessage not status.implemented yet.

	// Fetch the Cluster
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, elementalMachine.ObjectMeta)
	if err != nil {
		err := fmt.Errorf("fetching Cluster: %w", err)
		conditions.Set(elementalMachine, &clusterv1.Condition{
			Type:     infrastructurev1beta1.AssociationReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1beta1.MissingAssociatedClusterReason,
			Message:  err.Error(),
		})
		return ctrl.Result{}, err
	}

	// Reconciliation step #3: If the Cluster to which this resource belongs cannot be found, exit the reconciliation
	if cluster == nil {
		logger.Info("ElementalMachine resource is not associated with any Cluster")
		conditions.Set(elementalMachine, &clusterv1.Condition{
			Type:     infrastructurev1beta1.AssociationReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1beta1.MissingAssociatedClusterReason,
			Message:  "ElementalMachine resource is not associated with any Cluster",
		})
		return ctrl.Result{}, nil
	}

	if elementalMachine.GetDeletionTimestamp().IsZero() {
		// The object is not being deleted, so register the finalizer
		if !controllerutil.ContainsFinalizer(elementalMachine, infrastructurev1beta1.FinalizerElementalMachine) {
			// Reconciliation step #4: Add the provider-specific finalizer, if needed
			controllerutil.AddFinalizer(elementalMachine, infrastructurev1beta1.FinalizerElementalMachine)
		}
		// Reconciliation step #5: If the associated Cluster‘s status.infrastructureReady is false, exit the reconciliation
		if !cluster.Status.InfrastructureReady {
			logger.Info("Cluster status.infrastructureReady is false")
			conditions.Set(elementalMachine, &clusterv1.Condition{
				Type:     infrastructurev1beta1.AssociationReady,
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityError,
				Reason:   infrastructurev1beta1.MissingClusterInfrastructureReadyReason,
				Message:  "Cluster status.infrastructureReady is false",
			})
			return ctrl.Result{}, nil
		}
		// Reconciliation step #6: If the associated Machine‘s spec.bootstrap.dataSecretName is nil, exit the reconciliation
		if machine.Spec.Bootstrap.DataSecretName == nil {
			logger.Info("Machine spec.bootstrap.dataSecretName is nil")
			conditions.Set(elementalMachine, &clusterv1.Condition{
				Type:     infrastructurev1beta1.AssociationReady,
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityError,
				Reason:   infrastructurev1beta1.MissingBootstrapSecretReason,
				Message:  "Machine spec.bootstrap.dataSecretName is nil",
			})
			return ctrl.Result{}, nil
		}
		// Reconciliation step #7: Reconcile provider-specific machine infrastructure
		result, err := r.reconcileNormal(ctx, cluster, elementalMachine, *machine)
		if err != nil {
			// Reconciliation step #7-1: If they are terminal failures, set status.failureReason and status.failureMessage
			// TODO: Consider implementing status.failureReason and status.failureMessage
			return ctrl.Result{}, fmt.Errorf("reconciling ElementalMachine: %w", err)
		}
		return result, nil
	}

	// The object is up for deletion
	if controllerutil.ContainsFinalizer(elementalMachine, infrastructurev1beta1.FinalizerElementalMachine) {
		result, err := r.reconcileDelete(ctx, elementalMachine)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling ElementalMachine deletion: %w", err)
		}
		return result, err
	}

	return ctrl.Result{}, nil
}

func (r *ElementalMachineReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster, elementalMachine *infrastructurev1beta1.ElementalMachine, machine clusterv1.Machine) (ctrl.Result, error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, elementalMachine.Namespace).
		WithValues(ilog.KeyElementalMachine, elementalMachine.Name).
		WithValues(ilog.KeyCluster, cluster.Name)
	logger.Info("Normal ElementalMachine reconcile")
	// Always assume Ready false
	elementalMachine.Status.Ready = false

	// Reconciliation step #7-2: If this is a control plane machine, register the instance with the provider’s control plane load balancer (optional)
	// TODO: Not implemented yet.

	// elementalMachine.Spec.HostRef is used to mark a link between the ElementalMachine and an ElementalHost
	if elementalMachine.Spec.HostRef == nil {
		return r.associateElementalHost(ctx, elementalMachine, machine)
	}

	// Reconciliation step #9: Set status.ready to true
	host := &infrastructurev1beta1.ElementalHost{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: elementalMachine.Spec.HostRef.Namespace, Name: elementalMachine.Spec.HostRef.Name}, host)
	// If the ElementalHost was not found, assume it was deleted, for example due to hardware failure.
	// Re-association with a new host should happen for this ElementalMachine then.
	if apierrors.IsNotFound(err) {
		logger.Info("ElementalHost is not found. Removing association reference", ilog.KeyElementalHost, elementalMachine.Spec.HostRef.Name)
		conditions.Set(elementalMachine, &clusterv1.Condition{
			Type:     infrastructurev1beta1.AssociationReady,
			Status:   corev1.ConditionFalse,
			Severity: infrastructurev1beta1.AssociatedHostNotFoundReasonSeverity,
			Reason:   infrastructurev1beta1.AssociatedHostNotFoundReason,
			Message:  fmt.Sprintf("Previously associated host not found: %s", elementalMachine.Spec.HostRef.Name),
		})
		elementalMachine.Spec.ProviderID = nil
		elementalMachine.Spec.HostRef = nil
		return ctrl.Result{RequeueAfter: r.RequeuePeriod}, nil
	}
	if err != nil {
		err := fmt.Errorf("fetching associated ElementalHost '%s': %w", elementalMachine.Spec.HostRef.Name, err)
		conditions.Set(elementalMachine, &clusterv1.Condition{
			Type:     infrastructurev1beta1.AssociationReady,
			Status:   corev1.ConditionFalse,
			Severity: infrastructurev1beta1.AssociatedHostNotFoundReasonSeverity,
			Reason:   infrastructurev1beta1.AssociatedHostNotFoundReason,
			Message:  err.Error(),
		})
		// Do not remove the association. Assume this is a recoverable error (for ex. permissions or i/o)
		return ctrl.Result{RequeueAfter: r.RequeuePeriod}, err
	}
	// Since we invalidate AssociationReady when fetching the associated host and failing,
	// we must restore AssociationReady true status after recovery.
	conditions.Set(elementalMachine, &clusterv1.Condition{
		Type:     infrastructurev1beta1.AssociationReady,
		Status:   corev1.ConditionTrue,
		Severity: clusterv1.ConditionSeverityInfo,
	})
	logger = logger.WithValues(ilog.KeyElementalHost, host.Name)

	// Check if the Host is installed and Bootstrapped
	if value, found := host.Labels[infrastructurev1beta1.LabelElementalHostInstalled]; !found || value != "true" {
		logger.Info("Waiting for ElementalHost to be installed")
		conditions.Set(elementalMachine, &clusterv1.Condition{
			Type:     infrastructurev1beta1.HostReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1beta1.HostWaitingForInstallReason,
			Message:  fmt.Sprintf("ElementalHost '%s' is not installed.", host.Name),
		})
		return ctrl.Result{}, nil
	}
	if value, found := host.Labels[infrastructurev1beta1.LabelElementalHostBootstrapped]; !found || value != "true" {
		logger.Info("Waiting for ElementalHost to be bootstrapped")
		conditions.Set(elementalMachine, &clusterv1.Condition{
			Type:     infrastructurev1beta1.HostReady,
			Status:   corev1.ConditionFalse,
			Severity: infrastructurev1beta1.HostWaitingForBootstrapReasonSeverity,
			Reason:   infrastructurev1beta1.HostWaitingForBootstrapReason,
			Message:  fmt.Sprintf("Waiting for ElementalHost '%s' to be bootstrapped", host.Name),
		})
		return ctrl.Result{}, nil
	}

	// Mark the HostReady condition true.
	// This is different than setting the elementalMachine.Status.Ready flag.
	// It just highlights that there is nothing to do anymore on the host side.
	conditions.Set(elementalMachine, &clusterv1.Condition{
		Type:     infrastructurev1beta1.HostReady,
		Status:   corev1.ConditionTrue,
		Severity: clusterv1.ConditionSeverityInfo,
	})

	// Wait for the Cluster's ControlPlane to be initialized before setting the ProviderID
	// This controller will need to set the `node.spec.providerID` on the downstream cluster,
	// therefore we need a working control plane to access it.
	if cluster.Spec.ControlPlaneRef == nil ||
		(cluster.Spec.ControlPlaneRef != nil && !conditions.IsTrue(cluster, clusterv1.ControlPlaneInitializedCondition)) {
		logger.Info("Cluster's control plane is not initialized yet")
		conditions.Set(elementalMachine, &clusterv1.Condition{
			Type:     infrastructurev1beta1.ProviderIDReady,
			Status:   corev1.ConditionFalse,
			Severity: infrastructurev1beta1.WaitingForControlPlaneReasonSeverity,
			Reason:   infrastructurev1beta1.WaitingForControlPlaneReason,
			Message:  fmt.Sprintf("Waiting for downstream cluster '%s' control plane initialized.", cluster.Name),
		})
		return ctrl.Result{RequeueAfter: r.RequeuePeriod}, nil
	}

	// Set the ProviderID on both ElementalMachine and downstream node
	if err := r.setProviderID(ctx, elementalMachine, cluster); err != nil {
		return ctrl.Result{RequeueAfter: r.RequeuePeriod}, fmt.Errorf("setting ProviderID: %w", err)
	}

	// Mark the ElementalMachine as ready
	logger.Info("ElementalMachine is ready")
	elementalMachine.Status.Ready = true

	// Reconciliation step #11: Set spec.failureDomain to the provider-specific failure domain the instance is running in (optional)
	// TODO: Not implemented yet.
	return ctrl.Result{}, nil
}

// setProviderID updates the ProviderID on the ElementalMachine and on the equivalent downstream cluster node.
//
// See: https://cluster-api.sigs.k8s.io/developer/providers/machine-infrastructure
//
// providerID (string): the identifier for the provider’s machine instance.
// This field is expected to match the value set by the KCM cloud provider in the Nodes.
// The Machine controller bubbles it up to the Machine CR, and it’s used to find the matching Node.
// Any other consumers can use the providerID as the source of truth to match both Machines and Nodes.
func (r *ElementalMachineReconciler) setProviderID(ctx context.Context, elementalMachine *infrastructurev1beta1.ElementalMachine, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, elementalMachine.Namespace).
		WithValues(ilog.KeyElementalMachine, elementalMachine.Name).
		WithValues(ilog.KeyCluster, cluster.Name)
	logger.Info("Setting ProviderID")
	if elementalMachine.Spec.HostRef == nil {
		logger.Error(ErrMissingHostReference, "ElementalMachine HostRef not set yet")
		return ErrMissingHostReference
	}
	logger = logger.WithValues(ilog.KeyElementalHost, elementalMachine.Spec.HostRef.Name)

	providerID := fmt.Sprintf("elemental://%s/%s", elementalMachine.Spec.HostRef.Namespace, elementalMachine.Spec.HostRef.Name)

	// If the same ProviderID was already set on the ElementalMachine, assume there is nothing to do.
	if elementalMachine.Spec.ProviderID != nil && *elementalMachine.Spec.ProviderID == providerID {
		logger.V(ilog.DebugLevel).Info("ProviderID already set, nothing to do.", "providerID", providerID)
		return nil
	}

	// Set the ProviderID on the downstream cluster node
	logger.Info("Setting providerID on downstream node")
	err := r.Tracker.SetProviderID(ctx,
		client.ObjectKeyFromObject(cluster),
		elementalMachine.Spec.HostRef.Name,
		providerID)
	if errors.Is(err, utils.ErrRemoteNodeNotFound) {
		logger.Error(err, "Remote Node not found")
		conditions.Set(elementalMachine, &clusterv1.Condition{
			Type:     infrastructurev1beta1.ProviderIDReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   infrastructurev1beta1.NodeNotFoundReason,
			Message:  fmt.Sprintf("Downstream cluster node '%s' is not found.", elementalMachine.Spec.HostRef.Name),
		})
		return fmt.Errorf("setting provider ID on remote node '%s': %w", elementalMachine.Spec.HostRef.Name, err)
	}
	if err != nil {
		logger.Error(err, "Could not access remote Node")
		return fmt.Errorf("setting provider ID on remote node '%s': %w", elementalMachine.Spec.HostRef.Name, err)
	}

	// Reconciliation step #8: Set spec.providerID to the provider-specific identifier for the provider’s machine instance
	elementalMachine.Spec.ProviderID = &providerID
	conditions.Set(elementalMachine, &clusterv1.Condition{
		Type:     infrastructurev1beta1.ProviderIDReady,
		Status:   corev1.ConditionTrue,
		Severity: clusterv1.ConditionSeverityInfo,
	})
	logger.Info("ProviderID set successfully")
	return nil
}

func (r *ElementalMachineReconciler) associateElementalHost(ctx context.Context, elementalMachine *infrastructurev1beta1.ElementalMachine, machine clusterv1.Machine) (ctrl.Result, error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, elementalMachine.Namespace).
		WithValues(ilog.KeyElementalMachine, elementalMachine.Name)

	// Find available host for association
	elementalHostCandidate, err := r.findAvailableHost(ctx, *elementalMachine)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("finding available host for association: %w", err)
	}
	// If none available, try again later
	if elementalHostCandidate == nil {
		logger.Info("No ElementalHosts available for association. Waiting for new hosts to be provisioned.")
		conditions.Set(elementalMachine, &clusterv1.Condition{
			Type:     infrastructurev1beta1.AssociationReady,
			Status:   corev1.ConditionFalse,
			Severity: infrastructurev1beta1.MissingAvailableHostsReasonSeverity,
			Reason:   infrastructurev1beta1.MissingAvailableHostsReason,
			Message:  "No ElementalHosts available for association.",
		})
		return ctrl.Result{RequeueAfter: r.RequeuePeriod}, nil
	}
	logger = logger.WithValues(ilog.KeyElementalHost, elementalHostCandidate.Name)
	logger.Info("Available host found")

	if err := r.linkElementalHostToElementalMachine(ctx, machine, *elementalMachine, elementalHostCandidate); err != nil {
		return ctrl.Result{}, fmt.Errorf("linking ElementalHost to ElementalMachine: %w", err)
	}

	logger.Info("ElementalHost linked successfully")

	// Link the ElementalMachine to ElementalHost
	elementalMachine.Spec.HostRef = &corev1.ObjectReference{
		APIVersion: elementalHostCandidate.APIVersion,
		Kind:       elementalHostCandidate.Kind,
		Namespace:  elementalHostCandidate.Namespace,
		Name:       elementalHostCandidate.Name,
		UID:        elementalHostCandidate.UID,
	}

	conditions.Set(elementalMachine, &clusterv1.Condition{
		Type:     infrastructurev1beta1.AssociationReady,
		Status:   corev1.ConditionTrue,
		Severity: clusterv1.ConditionSeverityInfo,
	})

	// We already know the host is installed (from label selection) and the bootstrap secret is ready.
	// Therefore we can already set this condition.
	conditions.Set(elementalMachine, &clusterv1.Condition{
		Type:     infrastructurev1beta1.HostReady,
		Status:   corev1.ConditionFalse,
		Severity: infrastructurev1beta1.HostWaitingForBootstrapReasonSeverity,
		Reason:   infrastructurev1beta1.HostWaitingForBootstrapReason,
		Message:  fmt.Sprintf("Waiting for ElementalHost '%s' to be bootstrapped", elementalHostCandidate.Name),
	})

	return ctrl.Result{}, nil
}

func (r *ElementalMachineReconciler) linkElementalHostToElementalMachine(ctx context.Context, machine clusterv1.Machine, elementalMachine infrastructurev1beta1.ElementalMachine, elementalHostCandidate *infrastructurev1beta1.ElementalHost) error {
	// Create the patch helper.
	patchHelper, err := patch.NewHelper(elementalHostCandidate, r.Client)
	if err != nil {
		return fmt.Errorf("initializing patch helper: %w", err)
	}

	// Link the ElementalHost to ElementalMachine
	elementalHostCandidate.Spec.MachineRef = &corev1.ObjectReference{
		APIVersion: elementalMachine.APIVersion,
		Kind:       elementalMachine.Kind,
		Namespace:  elementalMachine.Namespace,
		Name:       elementalMachine.Name,
		UID:        elementalMachine.UID,
	}

	// Link Bootstrap Secret to ElementalHost
	elementalHostCandidate.Spec.BootstrapSecret = &corev1.ObjectReference{
		Kind:      "Secret",
		Namespace: machine.Namespace,
		Name:      *machine.Spec.Bootstrap.DataSecretName,
	}

	// Propagate the Machine and ElementalMachine names to ElementalHost
	elementalHostCandidate.Labels[infrastructurev1beta1.LabelElementalHostMachineName] = machine.Name
	elementalHostCandidate.Labels[infrastructurev1beta1.LabelElementalHostElementalMachineName] = elementalMachine.Name

	// Propagate the Cluster name to ElementalHost
	if name, ok := elementalMachine.Labels[clusterv1.ClusterNameLabel]; ok {
		elementalHostCandidate.Labels[clusterv1.ClusterNameLabel] = name
	}

	// Reconciliation step #10: Set status.addresses to the provider-specific set of instance addresses
	// TODO: Fetch the addresses from ElementalHost to update the associated ElementalMachine

	// Patch the associated ElementalHost
	if err := patchHelper.Patch(ctx, elementalHostCandidate); err != nil {
		return fmt.Errorf("patching ElementalHost: %w", err)
	}
	return nil
}

func (r *ElementalMachineReconciler) findAvailableHost(ctx context.Context, elementalMachine infrastructurev1beta1.ElementalMachine) (*infrastructurev1beta1.ElementalHost, error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, elementalMachine.Namespace).
		WithValues(ilog.KeyElementalMachine, elementalMachine.Name)
	logger.Info("Finding a suitable ElementalHost to associate")

	// First lookup ElementalHosts which may have been already linked before (ElementalMachine <-- ElementalHost).
	// This can happen if the association process stopped abruptly, before finalizing the ElementalMachine --> ElementalHost link.
	alreadyAssociatedHost, err := r.lookUpAlreadyLinkedHost(ctx, elementalMachine)
	if err != nil {
		return nil, fmt.Errorf("looking up already associated hosts: %w", err)
	}
	if alreadyAssociatedHost != nil {
		logger = logger.WithValues(ilog.KeyElementalHost, alreadyAssociatedHost.Name)
		logger.Info("Finalizing association with already linked ElementalHost")
		return alreadyAssociatedHost, nil
	}

	// If no already associated ElementalHost is found, find a new one.
	newHostCandidate, err := r.lookUpNewAvailableHost(ctx, elementalMachine)
	if err != nil {
		return nil, fmt.Errorf("looking up new available host: %w", err)
	}
	return newHostCandidate, nil
}

func (r *ElementalMachineReconciler) lookUpNewAvailableHost(ctx context.Context, elementalMachine infrastructurev1beta1.ElementalMachine) (*infrastructurev1beta1.ElementalHost, error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, elementalMachine.Namespace).
		WithValues(ilog.KeyElementalMachine, elementalMachine.Name)
	logger.Info("Looking up for a new ElementalHost to associate")

	elementalHosts := &infrastructurev1beta1.ElementalHostList{}
	var selector labels.Selector
	var err error

	// Use the label selector defined in the ElementalMachine, or select any ElementalHost available if no selector has been defined.
	if elementalMachine.Spec.Selector != nil {
		if selector, err = metav1.LabelSelectorAsSelector(elementalMachine.Spec.Selector); err != nil {
			return nil, fmt.Errorf("converting LabelSelector to Selector: %w", err)
		}
	} else {
		selector = labels.NewSelector()
	}

	// Select hosts that are Installed (all components installed, host ready to be bootstrapped)
	requirement, err := labels.NewRequirement(infrastructurev1beta1.LabelElementalHostInstalled, selection.Equals, []string{"true"})
	if err != nil {
		return nil, fmt.Errorf("adding host installed label requirement: %w", err)
	}
	selector = selector.Add(*requirement)
	// Select hosts that are not undergoing a Reset flow
	requirement, err = labels.NewRequirement(infrastructurev1beta1.LabelElementalHostNeedsReset, selection.DoesNotExist, nil)
	if err != nil {
		return nil, fmt.Errorf("adding host needs reset label requirement: %w", err)
	}
	selector = selector.Add(*requirement)
	// Select hosts that have not been associated yet
	requirement, err = labels.NewRequirement(infrastructurev1beta1.LabelElementalHostMachineName, selection.DoesNotExist, nil)
	if err != nil {
		return nil, fmt.Errorf("adding host machine name label requirement: %w", err)
	}
	selector = selector.Add(*requirement)

	// Query the available ElementalHosts within the same namespace as the ElementalMachine
	if err := r.Client.List(ctx, elementalHosts, client.InNamespace(elementalMachine.Namespace), &client.ListOptions{LabelSelector: selector}); err != nil {
		return nil, fmt.Errorf("listing available ElementalHosts: %w", err)
	}

	logger.WithCallDepth(ilog.DebugLevel).Info(fmt.Sprintf("Found %d available hosts", len(elementalHosts.Items)))

	// Return the first one available, if any
	for _, host := range elementalHosts.Items {
		return &host, nil
	}

	// No hosts available for association
	return nil, nil
}

func (r *ElementalMachineReconciler) lookUpAlreadyLinkedHost(ctx context.Context, elementalMachine infrastructurev1beta1.ElementalMachine) (*infrastructurev1beta1.ElementalHost, error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, elementalMachine.Namespace).
		WithValues(ilog.KeyElementalMachine, elementalMachine.Name)
	logger.Info("Looking up for an already linked ElementalHost to finalize association")

	elementalHosts := &infrastructurev1beta1.ElementalHostList{}
	selector := labels.NewSelector()

	requirement, err := labels.NewRequirement(infrastructurev1beta1.LabelElementalHostElementalMachineName, selection.Equals, []string{elementalMachine.Name})
	if err != nil {
		return nil, fmt.Errorf("adding elementalmachine name label requirement: %w", err)
	}
	selector = selector.Add(*requirement)
	// Also select hosts that are not undergoing a Reset flow
	requirement, err = labels.NewRequirement(infrastructurev1beta1.LabelElementalHostNeedsReset, selection.DoesNotExist, nil)
	if err != nil {
		return nil, fmt.Errorf("adding host needs reset label requirement: %w", err)
	}
	selector = selector.Add(*requirement)

	if err := r.Client.List(ctx, elementalHosts, client.InNamespace(elementalMachine.Namespace), &client.ListOptions{LabelSelector: selector}); err != nil {
		return nil, fmt.Errorf("listing previously linked ElementalHosts: %w", err)
	}

	logger.WithCallDepth(ilog.DebugLevel).Info(fmt.Sprintf("Found %d already linked hosts", len(elementalHosts.Items)))

	// If there is an already asssociated host, return it to finalize association.
	for _, host := range elementalHosts.Items {
		return &host, nil
	}

	return nil, nil
}

func (r *ElementalMachineReconciler) reconcileDelete(ctx context.Context, elementalMachine *infrastructurev1beta1.ElementalMachine) (ctrl.Result, error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, elementalMachine.Namespace).
		WithValues(ilog.KeyElementalMachine, elementalMachine.Name)
	logger.Info("Deletion ElementalMachine reconcile")

	// If the ElementalMachine was already associated to an ElementalHost, trigger the reset of such host.
	if elementalMachine.Spec.HostRef != nil {
		logger = logger.WithValues(ilog.KeyElementalHost, elementalMachine.Spec.HostRef.Name)
		logger.Info("Triggering Reset on associated ElementalHost")
		// Fetch the ElementalHost.
		host := &infrastructurev1beta1.ElementalHost{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Namespace: elementalMachine.Spec.HostRef.Namespace,
			Name:      elementalMachine.Spec.HostRef.Name,
		}, host); err != nil {
			if apierrors.IsNotFound(err) {
				// If the ElementalHost is not found, assume it's already deleted. Nothing to do.
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, fmt.Errorf("fetching ElementalHost: %w", err)
		}
		// Create the patch helper.
		patchHelper, err := patch.NewHelper(host, r.Client)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("initializing patch helper: %w", err)
		}

		// Mark this host for reset
		if host.Labels == nil {
			host.Labels = map[string]string{}
		}
		host.Labels[infrastructurev1beta1.LabelElementalHostNeedsReset] = "true"

		// Patch
		if err := patchHelper.Patch(ctx, host); err != nil {
			return ctrl.Result{}, fmt.Errorf("patching ElementalHost: %w", err)
		}
	}

	controllerutil.RemoveFinalizer(elementalMachine, infrastructurev1beta1.FinalizerElementalMachine)
	return ctrl.Result{}, nil
}
