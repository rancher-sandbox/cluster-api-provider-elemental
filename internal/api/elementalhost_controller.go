package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	infrastructurev1beta3 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta3"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/util/patch"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Server) PatchMachineHost(response http.ResponseWriter, request *http.Request) {
	pathVars := mux.Vars(request)
	namespace := pathVars["namespace"]
	registrationName := pathVars["registrationName"]
	hostName := pathVars["hostName"]

	// Fetch registration
	registration := &infrastructurev1beta3.ElementalMachineRegistration{}
	if err := s.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: namespace, Name: registrationName}, registration); err != nil {
		if k8sapierrors.IsNotFound(err) {
			response.WriteHeader(http.StatusNotFound)
			response.Write([]byte(fmt.Sprintf("ElementalMachineRegistration '%s' not found", registrationName)))
		} else {
			s.logger.Error(err, "Could not fetch ElementalMachineRegistration", "namespace", namespace, "registrationName", registrationName)
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(fmt.Sprintf("Could not fetch ElementalMachineRegistration '%s'", registrationName)))
		}
		return
	}

	// Fetch host
	host := &infrastructurev1beta3.ElementalHost{}
	if err := s.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: namespace, Name: hostName}, host); err != nil {
		if k8sapierrors.IsNotFound(err) {
			response.WriteHeader(http.StatusNotFound)
			response.Write([]byte(fmt.Sprintf("ElementalHost '%s' not found", hostName)))
		} else {
			s.logger.Error(err, "Could not fetch ElementalHost", "namespace", namespace, "hostName", hostName)
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(fmt.Sprintf("Could not fetch ElementalHost '%s'", hostName)))
		}
		return
	}

	// Unmarshal PATCH request body.
	hostPatch := &infrastructurev1beta3.ElementalMachineRegistration{}
	if err := json.NewDecoder(request.Body).Decode(hostPatch); err != nil {
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte(err.Error()))
		return
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(host, s.k8sClient)
	if err != nil {
		s.logger.Error(err, "Initializing ElementalHost patch helper", "namespace", namespace, "hostName", hostName)
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Could not initialize ElementalHost patch helper"))
		return
	}

	// Patch the object.
	if err := patchHelper.Patch(request.Context(), hostPatch); err != nil {
		s.logger.Error(err, "Could not patch ElementalHost", "namespace", namespace, "hostName", hostName)
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(fmt.Sprintf("Could not patch ElementalHost '%s'", hostName)))
		return
	}

	// Fetch the updated host
	host = &infrastructurev1beta3.ElementalHost{}
	if err := s.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: namespace, Name: hostName}, host); err != nil {
		if k8sapierrors.IsNotFound(err) {
			response.WriteHeader(http.StatusNotFound)
			response.Write([]byte(fmt.Sprintf("Updated ElementalHost '%s' not found", hostName)))
		} else {
			s.logger.Error(err, "Could not fetch updated ElementalHost", "namespace", namespace, "hostName", hostName)
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(fmt.Sprintf("Could not fetch updated ElementalHost '%s'", hostName)))
		}
		return
	}

	// Serialize to JSON
	responseBytes, err := json.Marshal(host)
	if err != nil {
		s.logger.Error(err, "Could not encode response body", "host", fmt.Sprintf("%+v", host))
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(fmt.Errorf("Could not encode response body: %w", err).Error()))
		return
	}

	response.Header().Add("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	response.Write(responseBytes)
}

func (s *Server) PostMachineHost(response http.ResponseWriter, request *http.Request) {
	pathVars := mux.Vars(request)
	namespace := pathVars["namespace"]
	registrationName := pathVars["registrationName"]

	// Fetch registration
	registration := &infrastructurev1beta3.ElementalMachineRegistration{}
	if err := s.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: namespace, Name: registrationName}, registration); err != nil {
		if k8sapierrors.IsNotFound(err) {
			response.WriteHeader(http.StatusNotFound)
			response.Write([]byte(fmt.Sprintf("ElementalMachineRegistration '%s' not found", registrationName)))
		} else {
			s.logger.Error(err, "Could not fetch ElementalMachineRegistration", "namespace", namespace, "registrationName", registrationName)
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(fmt.Sprintf("Could not fetch ElementalMachineRegistration '%s'", registrationName)))
		}
		return
	}

	// Unmarshal POST request body.
	newHost := &infrastructurev1beta3.ElementalHost{}
	if err := json.NewDecoder(request.Body).Decode(newHost); err != nil {
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte(fmt.Errorf("Could not decode request body: %w", err).Error()))
		return
	}

	// Validate new Host
	if err := validateNewHost(newHost, registration); err != nil {
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte(err.Error()))
		return
	}

	// Set Registration Owner
	newHost.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: registration.APIVersion,
			Kind:       registration.Kind,
			Name:       registration.Name,
			UID:        registration.UID,
			Controller: ptr.To(true),
		},
	}

	// Create new Host
	if err := s.k8sClient.Create(request.Context(), newHost); err != nil {
		if k8sapierrors.IsAlreadyExists(err) {
			response.WriteHeader(http.StatusConflict)
			response.Write([]byte(fmt.Sprintf("Host '%s' in namespace '%s' already exists", namespace, newHost.Name)))
		} else {
			s.logger.Error(err, "Could not create ElementalHost", "namespace", namespace, "hostName", newHost.Name)
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(fmt.Sprintf("Could not create Elemental Host '%s'", newHost.Name)))
		}
		return
	}

	response.WriteHeader(http.StatusCreated)
	response.Header().Add("Content-Type", "application/json")
	response.Header().Add("Location", fmt.Sprintf("%s%s/namespaces/%s/registrations/%s/hosts/%s", PrefixAPI, PrefixV1, namespace, registrationName, newHost.Name))
}

func validateNewHost(newHost *infrastructurev1beta3.ElementalHost, registration *infrastructurev1beta3.ElementalMachineRegistration) error {
	if newHost.Namespace != registration.Namespace {
		return errors.New("Invalid namespace")
	}
	// TODO: Add more to validate
	return nil
}
