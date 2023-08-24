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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	bootstraputil "k8s.io/cluster-bootstrap/token/util"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	infrastructurev1beta3 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta3"
)

// ElementalMachineRegistrationReconciler reconciles a ElementalMachineRegistration object
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
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ElementalMachineRegistration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *ElementalMachineRegistrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("elementalMachineRegistration", req.NamespacedName)
	logger.Info("Reconciling ElementalMachineRegistration")

	// Fetch the ElementalMachineRegistration
	elementalMachineRegistration := &infrastructurev1beta3.ElementalMachineRegistration{}
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
	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		patchHelper.Patch(ctx, elementalMachineRegistration)
	}()

	// If no Bootstrap token was generated yet, let's create one
	if elementalMachineRegistration.Spec.BootstrapTokenRef == nil {
		if err := r.generateBoostrapToken(ctx, logger, elementalMachineRegistration); err != nil {
			return ctrl.Result{}, fmt.Errorf("initializing bootstrap token secret: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// TODO: This entire logic should be moved to the SeedImage controller.
//
//		The bootstrap token is 1:1 coupled with an image.
//		For better security it would be best to generate different tokens for each different image,
//		for example when building for multiple architectures, versions, or when refreshing/rebuilding an expired image.
//		It's important to tie the token expiration to the image expiration, and set some sane defaults so that they will expire.
//
//	 See: https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/#bootstrap-token-secret-format
func (r *ElementalMachineRegistrationReconciler) generateBoostrapToken(ctx context.Context, logger logr.Logger, registration *infrastructurev1beta3.ElementalMachineRegistration) error {
	logger.Info("Generating new Bootstrap Token Secret")
	// Generate a valid token
	token, err := bootstraputil.GenerateBootstrapToken()
	if err != nil {
		return fmt.Errorf("generating bootstrap token: %w", err)
	}

	// Extract the ID and Secret (Format is "{tokenID}.{tokenSecret}")
	// TODO: Would be nice to have this in "k8s.io/cluster-bootstrap/token/util"
	tokenParts := strings.Split(token, ".")
	if len(tokenParts) < 2 {
		return errors.New("expected Bootstrap Token to be formatted as:'{tokenID}.{tokenSecret}'")
	}
	tokenID := tokenParts[0]
	tokenSecret := tokenParts[1]

	// Get valid secret name
	secretName := bootstraputil.BootstrapTokenSecretName(tokenID)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: registration.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: registration.APIVersion,
					Kind:       registration.Kind,
					Name:       registration.Name,
					UID:        registration.UID,
					Controller: pointer.Bool(true),
				},
			},
		},
		Type: corev1.SecretTypeBootstrapToken,
		StringData: map[string]string{
			"description":  "Elemental Bootstrap Token",
			"token-id":     tokenID,
			"token-secret": tokenSecret,
			// "expiration": "" //TODO: Would be great to implement expiration. This can be coupled to the seed image.
			"usage-bootstrap-authentication": `"true"`,
			"usage-bootstrap-signing":        `"true"`,
		},
	}

	if err := r.Client.Create(ctx, secret); err != nil {
		return fmt.Errorf("creating Bootstrap Token Secret: %w", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElementalMachineRegistrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta3.ElementalMachineRegistration{}).
		Complete(r)
}
