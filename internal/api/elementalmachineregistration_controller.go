package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/log"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Server) GetMachineRegistration(response http.ResponseWriter, request *http.Request) {
	pathVars := mux.Vars(request)
	namespace := pathVars["namespace"]
	registrationName := pathVars["registrationName"]

	logger := s.logger.WithValues(log.KeyNamespace, namespace).
		WithValues(log.KeyElementalMachineRegistration, registrationName)
	logger.Info("Getting ElementalMachineRegistration")

	// Fetch registration
	registration := &infrastructurev1beta1.ElementalMachineRegistration{}
	if err := s.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: namespace, Name: registrationName}, registration); err != nil {
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
