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
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/golang-jwt/jwt/v5"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	ilog "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/log"
)

var (
	ErrAPIEndpointNil = errors.New("API endpoint is nil, the controller was not initialized correctly")
	ErrNoPrivateKey   = errors.New("could not find 'privKey' value in registration secret")
)

// ElementalRegistrationReconciler reconciles a ElementalRegistration object.
type ElementalRegistrationReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	APIUrl        *url.URL
	DefaultCACert string
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElementalRegistrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta1.ElementalRegistration{}).
		Complete(r); err != nil {
		return fmt.Errorf("initializing ElementalRegistrationReconciler builder: %w", err)
	}
	return nil
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalregistrations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalregistrations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalregistrations/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *ElementalRegistrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, req.Namespace).
		WithValues(ilog.KeyElementalRegistration, req.Name)
	logger.Info("Reconciling ElementalRegistration")

	// Fetch the ElementalRegistration
	registration := &infrastructurev1beta1.ElementalRegistration{}
	if err := r.Client.Get(ctx, req.NamespacedName, registration); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching ElementalRegistration: %w", err)
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(registration, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("initializing patch helper: %w", err)
	}
	defer func() {
		if err := patchHelper.Patch(ctx, registration); err != nil {
			rerr = errors.Join(rerr, fmt.Errorf("patching ElementalRegistration: %w", err))
		}
	}()

	// Only set the URI if not set before or manually by the end user.
	if len(registration.Spec.Config.Elemental.Registration.URI) == 0 {
		logger.Info("Setting Registration URI")
		if err := r.setURI(registration); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating registration URI: %w", err)
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// Set default CA Cert if not set already and if we have a default one to trust.
	if len(registration.Spec.Config.Elemental.Registration.CACert) == 0 && len(r.DefaultCACert) > 0 {
		logger.Info("Setting default CACert")
		registration.Spec.Config.Elemental.Registration.CACert = r.DefaultCACert
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// Generate new token signing key if secret does not exists yet.
	if registration.Spec.PrivateKeyRef == nil {
		logger.Info("Generating new signing key")
		if err := r.generateNewIdentity(ctx, registration); err != nil {
			return ctrl.Result{}, fmt.Errorf("generating new identity: %w", err)
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// Generate new token if does not exist yet.
	if len(registration.Spec.Config.Elemental.Registration.Token) == 0 {
		logger.Info("Generating new registration token")
		if err := r.setNewToken(ctx, registration); err != nil {
			return ctrl.Result{}, fmt.Errorf("refreshing registration token: %w", err)
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ElementalRegistrationReconciler) setURI(registration *infrastructurev1beta1.ElementalRegistration) error {
	if r.APIUrl == nil {
		return ErrAPIEndpointNil
	}
	registration.Spec.Config.Elemental.Registration.URI = fmt.Sprintf("%s%s%s/namespaces/%s/registrations/%s",
		r.APIUrl.String(),
		api.Prefix,
		api.PrefixV1,
		registration.Namespace,
		registration.Name)
	return nil
}

func (r *ElementalRegistrationReconciler) generateNewIdentity(ctx context.Context, registration *infrastructurev1beta1.ElementalRegistration) error {
	id, err := identity.NewED25519Identity()
	if err != nil {
		return fmt.Errorf("generating new ed25519 identity: %w", err)
	}
	privKeyPem, err := id.Marshal()
	if err != nil {
		return fmt.Errorf("marshaling PEM key: %w", err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      registration.Name,
			Namespace: registration.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: registration.APIVersion,
					Kind:       registration.Kind,
					Name:       registration.Name,
					UID:        registration.UID,
					Controller: ptr.To(true),
				},
			},
		},
		StringData: map[string]string{
			"privKey": string(privKeyPem),
		},
	}
	// If the secret already exists, assume it was created by this controller already or directly by the user.
	if err := r.Client.Create(ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating new secret: %w", err)
	}
	registration.Spec.PrivateKeyRef = &corev1.ObjectReference{
		Kind:      secret.Kind,
		Name:      secret.Name,
		Namespace: secret.Namespace,
		UID:       secret.UID,
	}
	return nil
}

func (r *ElementalRegistrationReconciler) setNewToken(ctx context.Context, registration *infrastructurev1beta1.ElementalRegistration) error {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      registration.Name,
		Namespace: registration.Namespace,
	}, secret); err != nil {
		return fmt.Errorf("fetching signing key secret: %w", err)
	}

	privKeyPem, found := secret.Data["privKey"]
	if !found {
		return ErrNoPrivateKey
	}

	id := identity.Ed25519Identity{}
	if err := id.Unmarshal([]byte(privKeyPem)); err != nil {
		return fmt.Errorf("parsing private key PEM: %w", err)
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Issuer:    "ElementalRegistrationReconciler",
		Subject:   registration.Spec.Config.Elemental.Registration.URI,
		Audience:  []string{registration.Spec.Config.Elemental.Registration.URI},
	}
	if registration.Spec.Config.Elemental.Registration.TokenDuration != 0 {
		claims.ExpiresAt = jwt.NewNumericDate(now.Add(registration.Spec.Config.Elemental.Registration.TokenDuration))
	}
	token, err := id.Sign(claims)
	if err != nil {
		return fmt.Errorf("signing JWT claims: %w", err)
	}

	registration.Spec.Config.Elemental.Registration.Token = token
	return nil
}
