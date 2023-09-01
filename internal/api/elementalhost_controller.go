package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
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
	registration := &infrastructurev1beta1.ElementalMachineRegistration{}
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
	host := &infrastructurev1beta1.ElementalHost{}
	if err := s.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: namespace, Name: hostName}, host); err != nil {
		if k8sapierrors.IsNotFound(err) {
			response.WriteHeader(http.StatusNotFound)
			response.Write([]byte(fmt.Sprintf("ElementalHost '%s' not found", hostName)))
		} else {
			s.logger.Error(err, "Could not fetch ElementalHost", "namespace", namespace, "hostName", hostName) // FIXME: Why is logger not printing anything at all?
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(fmt.Sprintf("Could not fetch ElementalHost '%s'", hostName)))
		}
		return
	}

	// Unmarshal PATCH request body.
	hostPatchRequest := &HostPatchRequest{}
	if err := json.NewDecoder(request.Body).Decode(hostPatchRequest); err != nil {
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte(fmt.Errorf("Could not decode request: %w", err).Error()))
		return
	}

	// Validate PATCH request
	if hostPatchRequest.Bootstrapped != nil {
		if *hostPatchRequest.Bootstrapped && host.Spec.BootstrapSecret == nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte("Can't mark the Host as bootstrapped if no bootstrap secret has been associated yet."))
			return
		}
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
	hostPatchRequest.mergeWithElementalHost(host)
	if err := patchHelper.Patch(request.Context(), host); err != nil {
		s.logger.Error(err, "Could not patch ElementalHost", "namespace", namespace, "hostName", hostName)
		response.WriteHeader(http.StatusInternalServerError)
		// response.Write([]byte(fmt.Sprintf("Could not patch ElementalHost '%s'", hostName)))
		response.Write([]byte(err.Error()))
		return
	}

	// Fetch the updated host
	host = &infrastructurev1beta1.ElementalHost{}
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

	hostResponse := HostResponse{}
	hostResponse.fromElementalHost(*host)
	// Serialize to JSON
	responseBytes, err := json.Marshal(hostResponse)
	if err != nil {
		s.logger.Error(err, "Could not encode response body", "host", fmt.Sprintf("%+v", hostResponse))
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(fmt.Errorf("Could not encode response body: %w", err).Error()))
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	response.Write(responseBytes)
}

func (s *Server) PostMachineHost(response http.ResponseWriter, request *http.Request) {
	pathVars := mux.Vars(request)
	namespace := pathVars["namespace"]
	registrationName := pathVars["registrationName"]

	// Fetch registration
	registration := &infrastructurev1beta1.ElementalMachineRegistration{}
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
	hostCreateRequest := &HostCreateRequest{}
	if err := json.NewDecoder(request.Body).Decode(hostCreateRequest); err != nil {
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte(fmt.Errorf("Could not decode request body: %w", err).Error()))
		return
	}

	// Set Registration Owner
	newHost := hostCreateRequest.toElementalHost(namespace)
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
	if err := s.k8sClient.Create(request.Context(), &newHost); err != nil {
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

	response.Header().Set("Location", fmt.Sprintf("%s%s/namespaces/%s/registrations/%s/hosts/%s", PrefixAPI, PrefixV1, namespace, registrationName, newHost.Name))
	response.WriteHeader(http.StatusCreated)
}

func (s *Server) GetMachineHostBootstrap(response http.ResponseWriter, request *http.Request) {
	pathVars := mux.Vars(request)
	namespace := pathVars["namespace"]
	registrationName := pathVars["registrationName"]
	hostName := pathVars["hostName"]

	// Fetch registration
	registration := &infrastructurev1beta1.ElementalMachineRegistration{}
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
	host := &infrastructurev1beta1.ElementalHost{}
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

	// Check if there is any Bootstrap secret associated to this host
	if host.Spec.BootstrapSecret == nil {
		response.WriteHeader(http.StatusNotFound)
		response.Write([]byte("There is no associated boostrap secret yet"))
		return
	}

	// Fetch bootstrap secret
	bootstrapSecret := &corev1.Secret{}
	if err := s.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: host.Spec.BootstrapSecret.Namespace, Name: host.Spec.BootstrapSecret.Name}, bootstrapSecret); err != nil {
		if k8sapierrors.IsNotFound(err) {
			s.logger.Error(err, "Could not find expected Bootstrap secret", "namespace", namespace, "hostName", hostName, "secretName", host.Spec.BootstrapSecret.Name)
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte("Could not find expected Bootstrap secret"))
		} else {
			s.logger.Error(err, "Could not fetch Bootstrap secret", "namespace", namespace, "hostName", hostName)
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte("Could not fetch Bootstrap secret"))
		}
		return
	}

	// Encode response
	bootstrapResponse := &BootstrapResponse{}
	bootstrapResponse.fromSecret(bootstrapSecret)

	responseBytes, err := json.Marshal(bootstrapResponse)
	if err != nil {
		s.logger.Error(err, "Could not encode bootstrap response body")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(fmt.Errorf("Could not encode bootstrap response body: %w", err).Error()))
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	response.Write(responseBytes)
}
