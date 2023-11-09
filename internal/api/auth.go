package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

type Authenticator interface {
	ValidateHostRequest(*http.Request, *v1beta1.ElementalHost, *v1beta1.ElementalRegistration) error
	ValidateRegistrationRequest(*http.Request, *v1beta1.ElementalRegistration) error
}

func NewAuthenticator() Authenticator {
	return &authenticator{}
}

var _ Authenticator = (*authenticator)(nil)

type authenticator struct{}

func (a *authenticator) ValidateHostRequest(request *http.Request, host *v1beta1.ElementalHost, registration *v1beta1.ElementalRegistration) error {
	authValue := request.Header.Get("Authorization")
	if len(authValue) == 0 {
		return fmt.Errorf("missing 'Authorization' header: %w", ErrUnauthorized)
	}
	token, found := strings.CutPrefix(authValue, "Bearer ")
	if !found {
		return fmt.Errorf("not a 'Bearer' token: %w", ErrUnauthorized)
	}
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
		return fmt.Errorf("validating JWT token: %w: %w", err, ErrForbidden)
	}
	return nil
}

func (a *authenticator) ValidateRegistrationRequest(_ *http.Request, _ *v1beta1.ElementalRegistration) error {
	return nil
}
