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

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
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
		Watches(
			&infrastructurev1beta1.ElementalMachine{},
			handler.EnqueueRequestsFromMapFunc(r.ElementalMachineToElementalHost),
		).
		Complete(r); err != nil {
		return fmt.Errorf("initializing ElementalHostReconciler builder: %w", err)
	}
	return nil
}

func (r *ElementalHostReconciler) ElementalMachineToElementalHost(ctx context.Context, obj client.Object) []ctrl.Request { //nolint: dupl
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, obj.GetNamespace()).
		WithValues(ilog.KeyElementalMachine, obj.GetName())
	logger.Info("Enqueueing ElementalHost reconciliation from ElementalMachine")

	requests := []ctrl.Request{}

	// Verify we are actually handling a ElementalMachine object
	machine, ok := obj.(*infrastructurev1beta1.ElementalMachine)
	if !ok {
		logger.Error(ErrEnqueueing, fmt.Sprintf("Expected a ElementalMachine object, but got %T", obj))
		return []ctrl.Request{}
	}

	// Check the ElementalMachine was associated to any ElementalHost
	if machine.Spec.HostRef != nil {
		logger.Info("Adding ElementalHost to reconciliation request", ilog.KeyElementalMachine, machine.Spec.HostRef.Name)
		name := client.ObjectKey{Namespace: machine.Spec.HostRef.Namespace, Name: machine.Spec.HostRef.Name}
		requests = append(requests, ctrl.Request{NamespacedName: name})
	}

	return requests
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
		// Reconcile Summary Condition
		conditions.SetSummary(host)
		// Patch the resource
		if err := patchHelper.Patch(ctx, host); err != nil {
			rerr = errors.Join(rerr, fmt.Errorf("patching ElementalHost: %w", err))
		}
	}()

	// Init labels map
	if host.Labels == nil {
		host.Labels = map[string]string{}
	}

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

	if host.Spec.MachineRef != nil {
		if err := r.reconcileOSVersionManagement(ctx, host); err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling OSVersionManagement: %w", err)
		}
	}

	// Reconcile Registered/Installed Condition (if the host is installed, assume it is registered as well)
	if value, found := host.Labels[infrastructurev1beta1.LabelElementalHostInstalled]; found && value == "true" {
		conditions.Set(host, &v1beta1.Condition{
			Type:     infrastructurev1beta1.RegistrationReady,
			Status:   v1.ConditionTrue,
			Severity: v1beta1.ConditionSeverityInfo,
		})
		conditions.Set(host, &v1beta1.Condition{
			Type:     infrastructurev1beta1.InstallationReady,
			Status:   v1.ConditionTrue,
			Severity: v1beta1.ConditionSeverityInfo,
		})
	}

	// Reconcile Bootstrapped Condition
	if value, found := host.Labels[infrastructurev1beta1.LabelElementalHostBootstrapped]; found && value == "true" {
		conditions.Set(host, &v1beta1.Condition{
			Type:     infrastructurev1beta1.BootstrapReady,
			Status:   v1.ConditionTrue,
			Severity: v1beta1.ConditionSeverityInfo,
		})
	}

	return ctrl.Result{}, nil
}

func (r *ElementalHostReconciler) reconcileOSVersionManagement(ctx context.Context, host *infrastructurev1beta1.ElementalHost) error {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, host.Namespace).
		WithValues(ilog.KeyElementalHost, host.Name).
		WithValues(ilog.KeyElementalMachine, host.Spec.MachineRef.Name)
	logger.Info("Reconciling OSVersionManagement from associated ElementalMachine")
	machine := &infrastructurev1beta1.ElementalMachine{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: host.Spec.MachineRef.Namespace, Name: host.Spec.MachineRef.Name}, machine)
	// If the ElementalMachine was not found, assume it was deleted.
	// Best thing we can do is to trigger reset by deleting this ElementalHost.
	// This has chances to happen if an ElementalMachine is deleted and the finalizer is removed.
	if apierrors.IsNotFound(err) {
		logger.Error(err, "Associated ElementalMachine is not found. Triggering Host reset.")
		now := metav1.Now()
		host.SetDeletionTimestamp(&now)
		return fmt.Errorf("fetching non-existing ElementalMachine '%s': %w", host.Spec.MachineRef.Name, err)
	}
	if err != nil {
		return fmt.Errorf("fetching associated ElementalMachine '%s': %w", host.Spec.MachineRef.Name, err)
	}

	host.Spec.OSVersionManagement = machine.Spec.OSVersionManagement
	return nil
}

func (r *ElementalHostReconciler) reconcileDelete(ctx context.Context, host *infrastructurev1beta1.ElementalHost) (ctrl.Result, error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, host.Namespace).
		WithValues(ilog.KeyElementalHost, host.Name)
	logger.Info("Deletion ElementalHost reconcile")

	if value, found := host.Labels[infrastructurev1beta1.LabelElementalHostReset]; found && value == "true" {
		logger.Info("ElementalHost reset successful")
		controllerutil.RemoveFinalizer(host, infrastructurev1beta1.FinalizerElementalMachine)
		conditions.Set(host, &v1beta1.Condition{
			Type:     infrastructurev1beta1.ResetReady,
			Status:   v1.ConditionTrue,
			Severity: v1beta1.ConditionSeverityInfo,
		})
		return ctrl.Result{}, nil
	}

	if value, found := host.Labels[infrastructurev1beta1.LabelElementalHostNeedsReset]; !found || value != "true" {
		logger.Info("Triggering reset for to-be-deleted ElementalHost")
		host.Labels[infrastructurev1beta1.LabelElementalHostNeedsReset] = "true"
		conditions.Set(host, &v1beta1.Condition{
			Type:     infrastructurev1beta1.ResetReady,
			Status:   v1.ConditionFalse,
			Severity: infrastructurev1beta1.WaitingForResetReasonSeverity,
			Reason:   infrastructurev1beta1.WaitingForResetReason,
			Message:  "Waiting for remote host to reset",
		})
		return ctrl.Result{}, nil
	}

	logger.Info("Waiting for host to be reset")
	return ctrl.Result{}, nil
}
