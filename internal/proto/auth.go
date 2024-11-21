package proto

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	validatedHostKey = "ValidatedElementalHost"
)

var (
	ErrPermissionDenied = errors.New("Permission Denied")
)

type contextKey string

type Authenticator interface {
	CanGetElementalRegistration(registrationToken string, registrationPubKey []byte) (bool, error)
	CanCreateElementalHost(registrationToken, hostToken string, registrationPubKey, hostPubKey []byte) (bool, error)
	CanGetPatchDeleteElementalHost(hostToken string, hostPubKey []byte) (bool, error)
}

func NewAuthenticator() Authenticator {
	return &authenticator{}
}

type authenticator struct {
}

func (a *authenticator) CanGetElementalRegistration(registrationToken string, registrationPubKey []byte) (bool, error) {
	return true, nil
}
func (a *authenticator) CanCreateElementalHost(registrationToken, hostToken string, registrationPubKey, hostPubKey []byte) (bool, error) {
	return true, nil
}
func (a *authenticator) CanGetPatchDeleteElementalHost(hostToken string, hostPubKey []byte) (bool, error) {
	return true, nil
}

type AuthInterceptor interface {
}

func NewAuthInterceptor(k8sClient client.Client, logger logr.Logger) AuthInterceptor {
	return &authInterceptor{
		k8sClient:     k8sClient,
		logger:        logger,
		authenticator: NewAuthenticator(),
	}
}

type authInterceptor struct {
	k8sClient     client.Client
	logger        logr.Logger
	authenticator Authenticator
}

type wrappedHostStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s wrappedHostStream) Context() context.Context {
	return s.ctx
}

func (a *authInterceptor) InterceptStream(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	switch info.FullMethod {
	case "elemental/ReconcileHost":
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Errorf(codes.InvalidArgument, "Missing metadata")
		}
		authorization, found := md["authorization"]
		if !found || len(authorization) < 1 {
			return status.Errorf(codes.Unauthenticated, "Missing authorization header")
		}
		host, err := a.ValidateHostRequest(ss.Context(), authorization[0])
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "Permission Denied: %s", err.Error())
		}
		// Inject the validated ElementalHost in the wrapped stream context
		wrappedStream := wrappedHostStream{ServerStream: ss, ctx: context.WithValue(ss.Context(), contextKey(validatedHostKey), host)}
		return handler(srv, wrappedStream)
	default:
		a.logger.Info("Dropping unexpected call", "method", info.FullMethod)
		return status.Errorf(codes.Unimplemented, "Could not authenticate method: %s", info.FullMethod)
	}
}

func (a *authInterceptor) InterceptUnary(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	return nil, status.Errorf(codes.Unimplemented, "Uniplemented intecteptor")
}

func (a *authInterceptor) ValidateHostRequest(ctx context.Context, bearerToken string) (v1beta1.ElementalHost, error) {
	host := v1beta1.ElementalHost{}

	token, found := strings.CutPrefix(bearerToken, "Bearer ")
	if !found {
		return host, fmt.Errorf("not a 'Bearer' token")
	}

	_, err := jwt.Parse(token, func(parsedToken *jwt.Token) (any, error) {
		// Extract the subject from parsed token
		subject, err := parsedToken.Claims.GetSubject()
		if err != nil {
			return nil, fmt.Errorf("getting subject from token: %w", err)
		}

		subjectParts := strings.Split(subject, string(types.Separator))
		if len(subjectParts) < 2 {
			return nil, fmt.Errorf("parsing subject '%s': Bad format", subject)
		}

		// Fetch the ElementalHost from subject
		hostKey := client.ObjectKey{
			Namespace: subjectParts[0],
			Name:      subjectParts[1],
		}

		logger := a.logger.WithValues(log.KeyNamespace, hostKey.Namespace).
			WithValues(log.KeyElementalHost, hostKey.Name)

		if err := a.k8sClient.Get(ctx, hostKey, &host); err != nil {
			logger.Error(err, "Could not get ElementalHost")
			return nil, ErrPermissionDenied
		}

		// Verify signature using ElementalHost's PubKey
		signingAlg := parsedToken.Method.Alg()
		switch signingAlg {
		case "EdDSA":
			pubKey, err := jwt.ParseEdPublicKeyFromPEM([]byte(host.Spec.PubKey))
			if err != nil {
				logger.Error(err, "Could not parse ElementalHost.spec.PubKey")
				return nil, ErrPermissionDenied
			}
			return pubKey, nil
		default:
			logger.Error(err, "JWT is using unsupported signing algorithm", "JWT Signing Alg", signingAlg)
			return nil, ErrPermissionDenied
		}
	})
	if err != nil {
		return host, fmt.Errorf("validating JWT token: %w", err)
	}
	return host, nil
}
