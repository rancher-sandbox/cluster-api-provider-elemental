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
	"reflect"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
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
		For(&infrastructurev1.ElementalHost{}).
		Watches(
			&infrastructurev1.ElementalMachine{},
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
	machine, ok := obj.(*infrastructurev1.ElementalMachine)
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
	host := &infrastructurev1.ElementalHost{}
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
		if !controllerutil.ContainsFinalizer(host, infrastructurev1.FinalizerElementalMachine) {
			controllerutil.AddFinalizer(host, infrastructurev1.FinalizerElementalMachine)
		}

		// Reconcile ElementalHost
		result, err := r.reconcileNormal(ctx, host)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling ElementalHost: %w", err)
		}
		return result, nil
	}

	// The object is up for deletion
	if controllerutil.ContainsFinalizer(host, infrastructurev1.FinalizerElementalMachine) {
		result, err := r.reconcileDelete(ctx, host)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling ElementalHost deletion: %w", err)
		}
		return result, err
	}

	return ctrl.Result{}, nil
}

func (r *ElementalHostReconciler) reconcileNormal(ctx context.Context, host *infrastructurev1.ElementalHost) (ctrl.Result, error) {
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
	if value, found := host.Labels[infrastructurev1.LabelElementalHostInstalled]; found && value == "true" {
		conditions.Set(host, &v1beta1.Condition{
			Type:     infrastructurev1.RegistrationReady,
			Status:   v1.ConditionTrue,
			Severity: v1beta1.ConditionSeverityInfo,
		})
		conditions.Set(host, &v1beta1.Condition{
			Type:     infrastructurev1.InstallationReady,
			Status:   v1.ConditionTrue,
			Severity: v1beta1.ConditionSeverityInfo,
		})
	}

	// Reconcile Bootstrapped Condition
	if value, found := host.Labels[infrastructurev1.LabelElementalHostBootstrapped]; found && value == "true" {
		conditions.Set(host, &v1beta1.Condition{
			Type:     infrastructurev1.BootstrapReady,
			Status:   v1.ConditionTrue,
			Severity: v1beta1.ConditionSeverityInfo,
		})
	}

	return ctrl.Result{}, nil
}

func (r *ElementalHostReconciler) reconcileOSVersionManagement(ctx context.Context, host *infrastructurev1.ElementalHost) error {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, host.Namespace).
		WithValues(ilog.KeyElementalHost, host.Name).
		WithValues(ilog.KeyElementalMachine, host.Spec.MachineRef.Name)
	logger.Info("Reconciling OSVersionManagement from associated ElementalMachine")
	machine := &infrastructurev1.ElementalMachine{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: host.Spec.MachineRef.Namespace,
		Name:      host.Spec.MachineRef.Name,
	}, machine)
	if apierrors.IsNotFound(err) {
		logger.Info("Not going to reconcile OSVersionManagement for no longer existing ElementalMachine")
		return nil
	}
	if err != nil {
		return fmt.Errorf("fetching associated ElementalMachine '%s': %w", host.Spec.MachineRef.Name, err)
	}

	// If we have a different OS Version to reconcile, then set the `OSVersionReady` false.
	if !reflect.DeepEqual(host.Spec.OSVersionManagement, machine.Spec.OSVersionManagement) {
		// Propagate the OSVersionManagement data
		host.Spec.OSVersionManagement = machine.Spec.OSVersionManagement

		// Since we are already bootstrapped, mutating the OSVersionManagement should be a warning if the in-place-update label was not set to pending.
		if value, found := host.Labels[infrastructurev1.LabelElementalHostInPlaceUpdate]; found && value != infrastructurev1.InPlaceUpdatePending {
			conditions.Set(host, &v1beta1.Condition{
				Type:     infrastructurev1.OSVersionReady,
				Status:   v1.ConditionFalse,
				Severity: infrastructurev1.InPlaceUpdateNotPendingSeverity,
				Reason:   infrastructurev1.InPlaceUpdateNotPending,
				Message:  fmt.Sprintf("ElementalMachine %s OSVersionManagement mutated, but no in-place-upgrade is pending. Mutation will be ignored.", machine.Name),
			})
		} else {
			conditions.Set(host, &v1beta1.Condition{
				Type:     infrastructurev1.OSVersionReady,
				Status:   v1.ConditionFalse,
				Severity: infrastructurev1.WaitingOSReconcileReasonSeverity,
				Reason:   infrastructurev1.WaitingOSReconcileReason,
				Message:  fmt.Sprintf("ElementalMachine %s OSVersionManagement mutated", machine.Name),
			})
		}
	}
	return nil
}

func (r *ElementalHostReconciler) reconcileDelete(ctx context.Context, host *infrastructurev1.ElementalHost) (ctrl.Result, error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, host.Namespace).
		WithValues(ilog.KeyElementalHost, host.Name)
	logger.Info("Deletion ElementalHost reconcile")

	if value, found := host.Labels[infrastructurev1.LabelElementalHostReset]; found && value == "true" {
		logger.Info("ElementalHost reset successful")
		controllerutil.RemoveFinalizer(host, infrastructurev1.FinalizerElementalMachine)
		conditions.Set(host, &v1beta1.Condition{
			Type:     infrastructurev1.ResetReady,
			Status:   v1.ConditionTrue,
			Severity: v1beta1.ConditionSeverityInfo,
		})
		return ctrl.Result{}, nil
	}

	if value, found := host.Labels[infrastructurev1.LabelElementalHostNeedsReset]; !found || value != "true" {
		logger.Info("Triggering reset for to-be-deleted ElementalHost")
		host.Labels[infrastructurev1.LabelElementalHostNeedsReset] = "true"
		conditions.Set(host, &v1beta1.Condition{
			Type:     infrastructurev1.ResetReady,
			Status:   v1.ConditionFalse,
			Severity: infrastructurev1.WaitingForResetReasonSeverity,
			Reason:   infrastructurev1.WaitingForResetReason,
			Message:  "Waiting for remote host to reset",
		})
		return ctrl.Result{}, nil
	}

	logger.Info("Waiting for host to be reset")
	return ctrl.Result{}, nil
}
