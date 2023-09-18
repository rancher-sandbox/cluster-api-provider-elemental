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
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	ilog "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/log"
)

// ElementalMachineRegistrationReconciler reconciles a ElementalMachineRegistration object.
type ElementalMachineRegistrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalmachineregistrations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalmachineregistrations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalmachineregistrations/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *ElementalMachineRegistrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, req.Namespace).
		WithValues(ilog.KeyElementalMachineRegistration, req.Name)
	logger.Info("Reconciling ElementalMachineRegistration")

	// Fetch the ElementalMachineRegistration
	elementalMachineRegistration := &infrastructurev1beta1.ElementalMachineRegistration{}
	if err := r.Client.Get(ctx, req.NamespacedName, elementalMachineRegistration); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching ElementalMachineRegistration: %w", err)
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(elementalMachineRegistration, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("initializing patch helper: %w", err)
	}
	defer func() {
		if err := patchHelper.Patch(ctx, elementalMachineRegistration); err != nil {
			rerr = fmt.Errorf("patching ElementalMachineRegistration: %w", err)
		}
	}()

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElementalMachineRegistrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta1.ElementalMachineRegistration{}).
		Complete(r); err != nil {
		return fmt.Errorf("initializing ElementalMachineRegistrationReconciler builder: %w", err)
	}
	return nil
}
