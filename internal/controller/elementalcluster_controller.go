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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1beta3 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta3"
)

// ElementalClusterReconciler reconciles a ElementalCluster object
type ElementalClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElementalClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta3.ElementalCluster{}).
		// Reconciliation step #1: If the resource is externally managed, exit the reconciliation
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(log.FromContext(ctx))).
		Complete(r)
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
//
// For more details about the reconciliation loop, check the official CAPI documentation:
// - https://cluster-api.sigs.k8s.io/developer/providers/cluster-infrastructure
func (r *ElementalClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling ElementalCluster")

	// Fetch the ElementalCluster
	elementalCluster := &infrastructurev1beta3.ElementalCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, elementalCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching ElementalCluster: %w", err)
	}

	// Reconciliation step #2: If the resource does not have a Cluster owner, exit the reconciliation
	cluster, err := util.GetOwnerCluster(ctx, r.Client, elementalCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting Cluster owner: %w")
	}
	if cluster == nil {
		logger.Info("Current resource has no Cluster owner")
		return ctrl.Result{}, nil
	}

	// Reconciliation step #3: Add the provider-specific finalizer, if needed
	// Not needed yet

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(elementalCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("initing patch helper: %w", err)
	}
	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		// Reconciliation step #8: Patch the resource to persist changes
		patchHelper.Patch(ctx, elementalCluster)
	}()

	// Reconciliation step #4: Reconcile provider-specific cluster infrastructure
	if elementalCluster.GetDeletionTimestamp() == nil || elementalCluster.GetDeletionTimestamp().IsZero() {
		// The object is not being deleted, handle reconcile
		if err := r.reconcile(ctx, elementalCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling ElementalCluster: %w")
		}
	} else {
		// The object is up for deletion, handle deletion reconcile
		if err := r.reconcileDelete(ctx, elementalCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling ElementalCluster deletion: %w")
		}
	}

	// Reconciliation step #5: If the provider created a load balancer for the control plane, record its hostname or IP
	// TODO: No idea yet how to tackle this.
	//       Do we need to fetch the LoadBalancer external IP and port, if any?

	// Reconciliation step #6: Set status.ready to true
	elementalCluster.Status.Ready = true

	// Reconciliation step #7: Set status.failureDomains based on available provider failure domains (optional)
	// TODO: No idea yet.

	return ctrl.Result{}, nil
}

func (r *ElementalClusterReconciler) reconcile(ctx context.Context, elementalCluster *infrastructurev1beta3.ElementalCluster) error {
	// TODO: Most likely nothing.
	//       Steps #5, #6, and #7 may be moved here though.
	return nil
}

func (r *ElementalClusterReconciler) reconcileDelete(ctx context.Context, elementalCluster *infrastructurev1beta3.ElementalCluster) error {
	// TODO: Most likely nothing.
	//       Expect CAPI controller to delete the Machine objects as well and on cascade the owned ElementalMachines.
	//       ElementalMachine controller can handle infra deletion reconciliation (for ex. reset)
	return nil
}
