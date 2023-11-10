package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/log"
	"github.com/swaggest/openapi-go"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ OpenAPIDecoratedHandler = (*GetElementalRegistrationHandler)(nil)
var _ http.Handler = (*GetElementalRegistrationHandler)(nil)

type GetElementalRegistrationHandler struct {
	logger    logr.Logger
	k8sClient client.Client
	auth      Authenticator
}

func NewGetElementalRegistrationHandler(logger logr.Logger, k8sClient client.Client) *GetElementalRegistrationHandler {
	return &GetElementalRegistrationHandler{
		logger:    logger,
		k8sClient: k8sClient,
		auth:      NewAuthenticator(k8sClient, logger),
	}
}

func (h *GetElementalRegistrationHandler) SetupOpenAPIOperation(oc openapi.OperationContext) error {
	oc.SetSummary("Get ElementalRegistration")
	oc.SetDescription("This endpoint returns an ElementalRegistration.")

	oc.AddReqStructure(RegistrationGetRequest{})

	oc.AddRespStructure(RegistrationResponse{}, WithDecoration("Returns the ElementalRegistration", "application/json", http.StatusOK))
	oc.AddRespStructure(nil, WithDecoration("If the ElementalRegistration is not found", "text/html", http.StatusNotFound))
	oc.AddRespStructure(nil, WithDecoration("If the 'Registration-Authorization' header does not contain a Bearer token", "text/html", http.StatusUnauthorized))
	oc.AddRespStructure(nil, WithDecoration("If the 'Registration-Authorization' token is not valid", "text/html", http.StatusForbidden))
	oc.AddRespStructure(nil, WithDecoration("", "text/html", http.StatusInternalServerError))

	return nil
}

func (h *GetElementalRegistrationHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	pathVars := mux.Vars(request)
	namespace := html.EscapeString(pathVars["namespace"])
	registrationName := html.EscapeString(pathVars["registrationName"])

	logger := h.logger.WithValues(log.KeyNamespace, namespace).
		WithValues(log.KeyElementalRegistration, registrationName)
	logger.Info("Getting ElementalRegistration")

	// Fetch registration
	registration := &infrastructurev1beta1.ElementalRegistration{}
	if err := h.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: namespace, Name: registrationName}, registration); err != nil {
		if k8sapierrors.IsNotFound(err) {
			response.WriteHeader(http.StatusNotFound)
			WriteResponse(logger, response, fmt.Sprintf("ElementalRegistration '%s' not found", registrationName))
		} else {
			logger.Error(err, "Could not fetch ElementalRegistration")
			response.WriteHeader(http.StatusInternalServerError)
			WriteResponse(logger, response, fmt.Sprintf("Could not fetch ElementalRegistration '%s'", registrationName))
		}
		return
	}

	// Authenticate Registration token
	if err := h.auth.ValidateRegistrationRequest(request, response, registration); err != nil {
		if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden) {
			logger.Info("Registration request denied", "reason", err.Error())
			return
		}
		logger.Error(err, "Could not authenticate registration request")
		return
	}

	registrationResponse := RegistrationResponse{}
	registrationResponse.fromElementalRegistration(*registration)

	// Serialize to JSON
	responseBytes, err := json.Marshal(registrationResponse)
	if err != nil {
		logger.Error(err, "Could not encode response body")
		response.WriteHeader(http.StatusInternalServerError)
		WriteResponse(logger, response, fmt.Errorf("Could not encode response body: %w", err).Error())
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	WriteResponseBytes(logger, response, responseBytes)
}
