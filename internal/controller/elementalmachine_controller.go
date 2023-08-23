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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1beta3 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta3"
)

// ElementalMachineReconciler reconciles a ElementalMachine object
type ElementalMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElementalMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta3.ElementalMachine{}).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.ClusterToElementalMachines),
			// Reconciliation step #5: If the associated Cluster‘s status.infrastructureReady is false, exit the reconciliation
			// Note: This check should not be blocking any further delete reconciliation flows.
			// Note: This check should only be performed after appropriate owner references (if any) are updated.
			builder.WithPredicates(predicates.ClusterUnpausedAndInfrastructureReady(ctrl.LoggerFrom(ctx))),
		).
		Complete(r)
}

// ClusterToElementalMachines is a handler.ToRequestsFunc to be used to enqeue requests for reconciliation of ElementalMachines.
func (r *ElementalMachineReconciler) ClusterToElementalMachines(ctx context.Context, obj client.Object) []ctrl.Request {
	logger := log.FromContext(ctx).WithValues("cluster", obj.GetName())
	logger.Info("Enqueueing ElementalMachines reconciliation from Cluster")

	requests := []ctrl.Request{}

	// Verify we are actually handling a Cluster object
	cluster, ok := obj.(*clusterv1.Cluster)
	if !ok {
		errMsg := fmt.Sprintf("Expected a Cluster object, but got %T", obj)
		logger.Error(errors.New(errMsg), errMsg)
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
		logger = logger.WithValues("elementalMachine", capiMachineList.Items[i].Name)
		logger.Info("Adding ElementalMachine to reconciliation request")
		requests = append(requests, ctrl.Request{NamespacedName: name})
	}

	return requests
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalmachines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
// For more details about the reconciliation loop, check the official CAPI documentation:
// - https://cluster-api.sigs.k8s.io/developer/providers/machine-infrastructure
func (r *ElementalMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling ElementalMachine")

	// Fetch the ElementalMachine
	elementalMachine := &infrastructurev1beta3.ElementalMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, elementalMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching ElementalMachine: %w", err)
	}

	// Reconciliation step #1: If the resource does not have a Machine owner, exit the reconciliation
	machine, err := util.GetOwnerMachine(ctx, r.Client, elementalMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting Machine owner: %w")
	}
	if machine == nil {
		logger.Info("ElementalMachine resource has no Machine owner")
		return ctrl.Result{}, nil
	}

	// Reconciliation step #2: If the resource has status.failureReason or status.failureMessage set, exit the reconciliation
	// TODO: status.failureReason and failureMessage not status.implemented yet.

	// Fetch the Cluster
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, elementalMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("fetching Cluster: %w", err)
	}

	// Reconciliation step #3: If the Cluster to which this resource belongs cannot be found, exit the reconciliation
	if cluster == nil {
		logger.Info("ElementalMachine resource is not associated with any Cluster")
		return ctrl.Result{}, nil
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(elementalMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("initializing patch helper: %w", err)
	}
	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		// Reconciliation step #12: Patch the resource to persist changes
		patchHelper.Patch(ctx, elementalMachine)
	}()

	if elementalMachine.GetDeletionTimestamp() == nil || elementalMachine.GetDeletionTimestamp().IsZero() {
		// The object is not being deleted, so register the finalizer
		if !controllerutil.ContainsFinalizer(elementalMachine, infrastructurev1beta3.FinalizerElementalMachine) {
			// Reconciliation step #4: Add the provider-specific finalizer, if needed
			controllerutil.AddFinalizer(elementalMachine, infrastructurev1beta3.FinalizerElementalMachine)
			// Reconciliation step #5: If the associated Cluster‘s status.infrastructureReady is false, exit the reconciliation
			if !cluster.Status.InfrastructureReady {
				logger.Info("Cluster status.infrastructureReady is false")
				return ctrl.Result{}, nil
			}
			// Reconciliation step #6: If the associated Machine‘s spec.bootstrap.dataSecretName is nil, exit the reconciliation
			if machine.Spec.Bootstrap.DataSecretName == nil {
				logger.Info("Machine spec.bootstrap.dataSecretName is nil")
				return ctrl.Result{}, nil
			}
			// Reconciliation step #7: Reconcile provider-specific machine infrastructure
			if err := r.reconcile(ctx, elementalMachine); err != nil {
				// Reconciliation step #7-1: If they are terminal failures, set status.failureReason and status.failureMessage
				// TODO: Consider implementing status.failureReason and status.failureMessage
				return ctrl.Result{}, fmt.Errorf("reconciling ElementalMachine: %w", err)
			}
		}
	} else {
		// The object is up for deletion
		if controllerutil.ContainsFinalizer(elementalMachine, infrastructurev1beta3.FinalizerElementalMachine) {
			if err := r.reconcileDelete(ctx, elementalMachine); err != nil {
				return ctrl.Result{}, fmt.Errorf("reconciling ElementalMachine deletion: %w", err)
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *ElementalMachineReconciler) reconcile(ctx context.Context, elementalMachine *infrastructurev1beta3.ElementalMachine) error {
	// Reconciliation step #7-2: If this is a control plane machine, register the instance with the provider’s control plane load balancer (optional)
	// TODO: Not implemented yet.

	if err := r.associateElementalHost(ctx, elementalMachine); err != nil {
		return fmt.Errorf("associating ElementalMachine to a suitable ElementalHost: %w", err)
	}

	// Reconciliation step #9: Set status.ready to true
	elementalMachine.Status.Ready = true

	// Reconciliation step #11: Set spec.failureDomain to the provider-specific failure domain the instance is running in (optional)
	// TODO: Not implemented yet.

	return nil
}

func (r *ElementalMachineReconciler) associateElementalHost(ctx context.Context, elementalMachine *infrastructurev1beta3.ElementalMachine) error {
	logger := log.FromContext(ctx).WithValues("elementalMachine", elementalMachine.Name)
	// elementalMachine.Spec.ProviderID is used to mark a link between the ElementalMachine and an ElementalHost
	// If this ElementalMachine was already associated, we have nothing to do.
	// TODO: Actually, we may as well check the ElementalHost status to update the ElementalMachine status as well.
	if elementalMachine.Spec.ProviderID == nil {
		logger.Info("Finding a suitable ElementalHost to associate")
		elementalHosts := &infrastructurev1beta3.ElementalHostList{}
		var selector labels.Selector
		var selectorErr error
		// Use the label selector defined in the ElementalMachine, or select any ElementalHost available if no selector has been defined.
		if elementalMachine.Spec.Selector != nil {
			if selector, selectorErr = metav1.LabelSelectorAsSelector(elementalMachine.Spec.Selector); selectorErr != nil {
				return fmt.Errorf("converting LabelSelector to Selector: %w", selectorErr)
			}
		} else {
			selector = labels.NewSelector()
		}

		if err := r.Client.List(ctx, elementalHosts, &client.ListOptions{LabelSelector: selector}); err != nil {
			return fmt.Errorf("listing available ElementalHosts: %w", err)
		}

		if len(elementalHosts.Items) == 0 {
			logger.Info("No ElementalHosts available for association. Waiting for new hosts to be provisioned.")
			return nil
		}

		// Just pick the first in the list
		elementalHostCandidate := elementalHosts.Items[0]
		logger = logger.WithValues("elementalHost", elementalHostCandidate.Name)
		logger.Info("Associating ElementalMachine to ElementalHost")

		// Reconciliation step #8: Set spec.providerID to the provider-specific identifier for the provider’s machine instance
		providerID := fmt.Sprintf("elemental://%s/%s", elementalHostCandidate.Namespace, elementalHostCandidate.Name)
		elementalMachine.Spec.ProviderID = &providerID

		// Create the patch helper.
		patchHelper, err := patch.NewHelper(&elementalHostCandidate, r.Client)
		if err != nil {
			return fmt.Errorf("initializing patch helper: %w", err)
		}

		// TODO: Decorate the ElementalHost with useful labels, for example the Cluster name, Control Plane endpoint, etc.

		// Reconciliation step #10: Set status.addresses to the provider-specific set of instance addresses
		// TODO: Fetch the addresses from ElementalHost to update the associated ElementalMachine

		// Patch the associated ElementalHost
		patchHelper.Patch(ctx, &elementalHostCandidate)

		logger.Info("Association successful")
	}
	return nil
}

func (r *ElementalMachineReconciler) reconcileDelete(ctx context.Context, elementalCluster *infrastructurev1beta3.ElementalMachine) error {

	return nil
}
