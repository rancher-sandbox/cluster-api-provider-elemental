package api

import (
	"encoding/json"
	"fmt"
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
}

func NewGetElementalRegistrationHandler(logger logr.Logger, k8sClient client.Client) *GetElementalRegistrationHandler {
	return &GetElementalRegistrationHandler{
		logger:    logger,
		k8sClient: k8sClient,
	}
}

func (h *GetElementalRegistrationHandler) SetupOpenAPIOperation(oc openapi.OperationContext) error {
	oc.SetSummary("Get ElementalRegistration")
	oc.SetDescription("This endpoint returns an ElementalRegistration.")

	oc.AddReqStructure(RegistrationGetRequest{})

	oc.AddRespStructure(RegistrationResponse{}, WithDecoration("Returns the ElementalRegistration", "application/json", http.StatusOK))
	oc.AddRespStructure(nil, WithDecoration("If the ElementalRegistration is not found", "text/html", http.StatusNotFound))
	oc.AddRespStructure(nil, WithDecoration("", "text/html", http.StatusInternalServerError))

	return nil
}

func (h *GetElementalRegistrationHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	pathVars := mux.Vars(request)
	namespace := pathVars["namespace"]
	registrationName := pathVars["registrationName"]

	logger := h.logger.WithValues(log.KeyNamespace, namespace).
		WithValues(log.KeyElementalMachineRegistration, registrationName)
	logger.Info("Getting ElementalMachineRegistration")

	// Fetch registration
	registration := &infrastructurev1beta1.ElementalMachineRegistration{}
	if err := h.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: namespace, Name: registrationName}, registration); err != nil {
		if k8sapierrors.IsNotFound(err) {
			response.WriteHeader(http.StatusNotFound)
			WriteResponse(logger, response, fmt.Sprintf("ElementalMachineRegistration '%s' not found", registrationName))
		} else {
			logger.Error(err, "Could not fetch ElementalMachineRegistration")
			response.WriteHeader(http.StatusInternalServerError)
			WriteResponse(logger, response, fmt.Sprintf("Could not fetch ElementalMachineRegistration '%s'", registrationName))
		}
		return
	}

	registrationResponse := RegistrationResponse{}
	registrationResponse.fromElementalMachineRegistration(*registration)

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
