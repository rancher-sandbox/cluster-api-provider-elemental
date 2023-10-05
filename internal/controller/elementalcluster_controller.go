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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	ilog "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/log"
)

// ElementalClusterReconciler reconciles a ElementalCluster object.
type ElementalClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElementalClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta1.ElementalCluster{}).
		// Reconciliation step #1: If the resource is externally managed, exit the reconciliation
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(log.FromContext(ctx))).
		Complete(r); err != nil {
		return fmt.Errorf("initializing ElementalClusterReconciler builder: %w", err)
	}
	return nil
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
//
// For more details about the reconciliation loop, check the official CAPI documentation:
// - https://cluster-api.sigs.k8s.io/developer/providers/cluster-infrastructure
func (r *ElementalClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, req.Namespace).
		WithValues(ilog.KeyElementalCluster, req.Name)
	logger.Info("Reconciling ElementalCluster")

	// Fetch the ElementalCluster
	elementalCluster := &infrastructurev1beta1.ElementalCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, elementalCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching ElementalCluster: %w", err)
	}

	// Reconciliation step #2: If the resource does not have a Cluster owner, exit the reconciliation
	cluster, err := util.GetOwnerCluster(ctx, r.Client, elementalCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting Cluster owner: %w", err)
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
		return ctrl.Result{}, fmt.Errorf("initializing patch helper: %w", err)
	}
	defer func() {
		// Reconciliation step #8: Patch the resource to persist changes
		if err := patchHelper.Patch(ctx, elementalCluster); err != nil {
			rerr = errors.Join(rerr, fmt.Errorf("patching ElementalCluster: %w", err))
		}
	}()

	// Reconciliation step #4: Reconcile provider-specific cluster infrastructure
	if elementalCluster.GetDeletionTimestamp() == nil || elementalCluster.GetDeletionTimestamp().IsZero() {
		// The object is not being deleted, handle reconcile
		if err := r.reconcileNormal(ctx, elementalCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling ElementalCluster: %w", err)
		}
	} else {
		// The object is up for deletion, handle deletion reconcile
		if err := r.reconcileDelete(ctx, elementalCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling ElementalCluster deletion: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *ElementalClusterReconciler) reconcileNormal(ctx context.Context, elementalCluster *infrastructurev1beta1.ElementalCluster) error {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, elementalCluster.Namespace).
		WithValues(ilog.KeyElementalCluster, elementalCluster.Name)
	logger.Info("Normal ElementalCluster reconcile")
	// Reconciliation step #5: If the provider created a load balancer for the control plane, record its hostname or IP
	// TODO: If using kube-vip, most likely nothing to do.
	//       However if no controlPlaneEndpoint was provided by the user, this could be an error.

	// Reconciliation step #6: Set status.ready to true
	elementalCluster.Status.Ready = true

	// Reconciliation step #7: Set status.failureDomains based on available provider failure domains (optional)
	// TODO: Considering implementing failure domains.
	return nil
}

func (r *ElementalClusterReconciler) reconcileDelete(ctx context.Context, elementalCluster *infrastructurev1beta1.ElementalCluster) error {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, elementalCluster.Namespace).
		WithValues(ilog.KeyElementalCluster, elementalCluster.Name)
	logger.Info("Delete ElementalCluster reconcile")
	// TODO: Most likely nothing.
	//       Expect CAPI controller to delete the Machine objects as well and on cascade the owned ElementalMachines.
	//       ElementalMachine controller can handle infra deletion reconciliation (for ex. reset)
	return nil
}
