package api

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrUnauthorized              = errors.New("unauthorized")
	ErrForbidden                 = errors.New("forbidden")
	ErrMissingRegistrationSecret = errors.New("registration secret is missing")
	ErrNoSigningKey              = errors.New("registration signing key is missing")
	ErrUnparsableSigningKey      = errors.New("registration signing key is not in the expected format")
)

type Authenticator interface {
	ValidateHostRequest(*http.Request, http.ResponseWriter, *v1beta1.ElementalHost, *v1beta1.ElementalRegistration) error
	ValidateRegistrationRequest(*http.Request, http.ResponseWriter, *v1beta1.ElementalRegistration) error
}

func NewAuthenticator(k8sClient client.Client, logger logr.Logger) Authenticator {
	return &authenticator{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

var _ Authenticator = (*authenticator)(nil)

type authenticator struct {
	logger    logr.Logger
	k8sClient client.Client
}

func (a *authenticator) ValidateHostRequest(request *http.Request, response http.ResponseWriter, host *v1beta1.ElementalHost, registration *v1beta1.ElementalRegistration) error {
	// Verify token was passed correctly
	authValue := request.Header.Get("Authorization")
	if len(authValue) == 0 {
		err := fmt.Errorf("missing 'Authorization' header: %w", ErrUnauthorized)
		a.writeResponse(response, err)
		return err
	}
	token, found := strings.CutPrefix(authValue, "Bearer ")
	if !found {
		err := fmt.Errorf("not a 'Bearer' token: %w", ErrUnauthorized)
		a.writeResponse(response, err)
		return err
	}
	// Validate and Verify JWT
	expectedClaims := &jwt.RegisteredClaims{
		Subject:  host.Name,
		Audience: []string{registration.Spec.Config.Elemental.Registration.URI},
	}
	_, err := jwt.ParseWithClaims(token, expectedClaims, func(parsedToken *jwt.Token) (any, error) {
		signingAlg := parsedToken.Method.Alg()
		switch signingAlg {
		case "EdDSA":
			pubKey, err := jwt.ParseEdPublicKeyFromPEM([]byte(host.Spec.PubKey))
			if err != nil {
				return nil, fmt.Errorf("parsing host Public Key: %w", err)
			}
			return pubKey, nil
		default:
			return nil, fmt.Errorf("JWT is using unsupported '%s' signing algorithm: %w", signingAlg, ErrUnauthorized)
		}
	})
	if err != nil {
		err := fmt.Errorf("validating JWT token: %w: %w", err, ErrForbidden)
		a.writeResponse(response, err)
		return err
	}
	return nil
}

func (a *authenticator) ValidateRegistrationRequest(request *http.Request, response http.ResponseWriter, registration *v1beta1.ElementalRegistration) error {
	// Verify token was passed correctly
	authValue := request.Header.Get("Registration-Authorization")
	if len(authValue) == 0 {
		err := fmt.Errorf("missing 'Registration-Authorization' header: %w", ErrUnauthorized)
		a.writeResponse(response, err)
		return err
	}
	token, found := strings.CutPrefix(authValue, "Bearer ")
	if !found {
		err := fmt.Errorf("not a 'Bearer' token: %w", ErrUnauthorized)
		a.writeResponse(response, err)
		return err
	}

	// Fetch registration secret and read the private key
	registrationSecret := &corev1.Secret{}
	if err := a.k8sClient.Get(request.Context(), types.NamespacedName{
		Name:      registration.Name,
		Namespace: registration.Namespace,
	}, registrationSecret); err != nil {
		err := fmt.Errorf("getting registration secret: %w", ErrMissingRegistrationSecret)
		a.writeResponse(response, err)
		return err
	}
	privKeyPem, found := registrationSecret.Data["privKey"]
	if !found {
		a.writeResponse(response, ErrNoSigningKey)
		return ErrNoSigningKey
	}
	parsedKey, err := jwt.ParseEdPrivateKeyFromPEM(privKeyPem)
	if err != nil {
		err := fmt.Errorf("parsing ed25519 key: %w", err)
		a.writeResponse(response, err)
		return err
	}
	var privKey ed25519.PrivateKey
	var ok bool
	if privKey, ok = parsedKey.(ed25519.PrivateKey); !ok {
		a.writeResponse(response, jwt.ErrNotEdPrivateKey)
		return jwt.ErrNotEdPrivateKey
	}
	// Validate and Verify JWT
	expectedClaims := &jwt.RegisteredClaims{
		Subject:  registration.Spec.Config.Elemental.Registration.URI,
		Audience: []string{registration.Spec.Config.Elemental.Registration.URI},
	}
	_, err = jwt.ParseWithClaims(token, expectedClaims, func(parsedToken *jwt.Token) (any, error) {
		signingAlg := parsedToken.Method.Alg()
		switch signingAlg {
		case "EdDSA":
			return privKey.Public(), nil
		default:
			return nil, fmt.Errorf("JWT is using unsupported '%s' signing algorithm: %w", signingAlg, ErrUnauthorized)
		}
	})
	if err != nil {
		err := fmt.Errorf("validating JWT token: %w: %w", err, ErrForbidden)
		a.writeResponse(response, err)
		return err
	}
	return nil
}

func (a *authenticator) writeResponse(response http.ResponseWriter, err error) {
	if errors.Is(err, ErrUnauthorized) {
		response.WriteHeader(http.StatusUnauthorized)
		WriteResponse(a.logger, response, fmt.Sprintf("Unauthorized: %s", err.Error()))
		return
	}
	if errors.Is(err, ErrForbidden) {
		response.WriteHeader(http.StatusForbidden)
		WriteResponse(a.logger, response, fmt.Sprintf("Forbidden: %s", err.Error()))
		return
	}
	response.WriteHeader(http.StatusInternalServerError)
	WriteResponse(a.logger, response, fmt.Sprintf("Could not authenticate request: %s", err.Error()))
	return
}
