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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	ilog "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/log"
)

// ElementalHostReconciler reconciles a ElementalHost object.
type ElementalHostReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElementalHostReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta1.ElementalHost{}).
		Complete(r); err != nil {
		return fmt.Errorf("initializing ElementalHostReconciler builder: %w", err)
	}
	return nil
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalhosts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalhosts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalhosts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *ElementalHostReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, req.Namespace).
		WithValues(ilog.KeyElementalHost, req.Name)
	logger.Info("Reconciling ElementalHost")

	// Fetch the ElementalHost
	host := &infrastructurev1beta1.ElementalHost{}
	if err := r.Client.Get(ctx, req.NamespacedName, host); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching ElementalHost: %w", err)
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(host, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("initializing patch helper: %w", err)
	}
	defer func() {
		if err := patchHelper.Patch(ctx, host); err != nil {
			rerr = errors.Join(rerr, fmt.Errorf("patching ElementalHost: %w", err))
		}
	}()

	// The object is not up for deletion
	if host.GetDeletionTimestamp() == nil || host.GetDeletionTimestamp().IsZero() {
		// The object is not being deleted, so register the finalizer
		if !controllerutil.ContainsFinalizer(host, infrastructurev1beta1.FinalizerElementalMachine) {
			controllerutil.AddFinalizer(host, infrastructurev1beta1.FinalizerElementalMachine)
		}

		// Reconcile ElementalHost
		result, err := r.reconcileNormal(ctx, host)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling ElementalHost: %w", err)
		}
		return result, nil
	}

	// The object is up for deletion
	if controllerutil.ContainsFinalizer(host, infrastructurev1beta1.FinalizerElementalMachine) {
		result, err := r.reconcileDelete(ctx, host)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling ElementalHost deletion: %w", err)
		}
		return result, err
	}

	return ctrl.Result{}, nil
}

func (r *ElementalHostReconciler) reconcileNormal(ctx context.Context, host *infrastructurev1beta1.ElementalHost) (ctrl.Result, error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, host.Namespace).
		WithValues(ilog.KeyElementalHost, host.Name)
	logger.Info("Normal ElementalHost reconcile")
	return ctrl.Result{}, nil
}

func (r *ElementalHostReconciler) reconcileDelete(ctx context.Context, host *infrastructurev1beta1.ElementalHost) (ctrl.Result, error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, host.Namespace).
		WithValues(ilog.KeyElementalHost, host.Name)
	logger.Info("Deletion ElementalHost reconcile")

	if host.Status.MachineRef != nil {
		logger = logger.WithValues(ilog.KeyElementalMachine, host.Status.MachineRef.Name)
		logger.Info("ElementalHost is associated to an ElementalMachine")
		elementalMachine := &infrastructurev1beta1.ElementalMachine{}
		err := r.Client.Get(ctx, types.NamespacedName{
			Name:      host.Status.MachineRef.Name,
			Namespace: host.Status.MachineRef.Namespace,
		}, elementalMachine)
		if apierrors.IsNotFound(err) {
			logger.Info("ElementalMachine was not found. Assuming deleted.")
			host.Status.MachineRef = nil
			return ctrl.Result{RequeueAfter: defaultRequeuePeriod}, nil
		}
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("fetching associated ElementalMachine: %w", err)
		}

		if elementalMachine.Status.HostRef != nil &&
			elementalMachine.Status.HostRef.Name == host.Name &&
			elementalMachine.Status.HostRef.Namespace == host.Namespace {
			logger.Info("ElementalMachine is associated to this ElementalHost. Removing reference.")
			patchHelper, err := patch.NewHelper(elementalMachine, r.Client)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("initializing patch helper: %w", err)
			}
			elementalMachine.Status.HostRef = nil
			elementalMachine.Spec.ProviderID = nil
			if err := patchHelper.Patch(ctx, host); err != nil {
				return ctrl.Result{}, fmt.Errorf("patching ElementalMachine: %w", err)
			}
		}
	}

	if !host.Status.NeedsReset {
		logger.Info("Triggering ElementalHost reset")
		host.Status.NeedsReset = true
		return ctrl.Result{RequeueAfter: defaultRequeuePeriod}, nil
	}

	if host.Status.Reset {
		logger.Info("ElementalHost reset successful")
		controllerutil.RemoveFinalizer(host, infrastructurev1beta1.FinalizerElementalMachine)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: defaultRequeuePeriod}, nil
}
