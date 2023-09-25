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
	corev1 "k8s.io/api/core/v1"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ OpenAPIDecoratedHandler = (*PatchElementalHostHandler)(nil)
var _ http.Handler = (*PatchElementalHostHandler)(nil)

type PatchElementalHostHandler struct {
	logger    logr.Logger
	k8sClient client.Client
}

func NewPatchElementalHostHandler(logger logr.Logger, k8sClient client.Client) *PatchElementalHostHandler {
	return &PatchElementalHostHandler{
		logger:    logger,
		k8sClient: k8sClient,
	}
}

func (h *PatchElementalHostHandler) SetupOpenAPIOperation(oc openapi.OperationContext) error {
	oc.SetSummary("Patch ElementalHost")
	oc.SetDescription("This endpoint patches an existing ElementalHost.")

	oc.AddReqStructure(HostPatchRequest{})

	oc.AddRespStructure(HostResponse{}, WithDecoration("Returns the patched ElementalHost", "application/json", http.StatusOK))
	oc.AddRespStructure(nil, WithDecoration("If the ElementalRegistration or the ElementalHost are not found", "text/html", http.StatusNotFound))
	oc.AddRespStructure(nil, WithDecoration("If the ElementalHostPatch request is badly formatted", "text/html", http.StatusBadRequest))
	oc.AddRespStructure(nil, WithDecoration("", "text/html", http.StatusInternalServerError))

	return nil
}

func (h *PatchElementalHostHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	pathVars := mux.Vars(request)
	namespace := pathVars["namespace"]
	registrationName := pathVars["registrationName"]
	hostName := pathVars["hostName"]

	logger := h.logger.WithValues(log.KeyNamespace, namespace).
		WithValues(log.KeyElementalMachineRegistration, registrationName).
		WithValues(log.KeyElementalHost, hostName)
	logger.Info("Patching ElementalHost")

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

	// Fetch host
	host := &infrastructurev1beta1.ElementalHost{}
	if err := h.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: namespace, Name: hostName}, host); err != nil {
		if k8sapierrors.IsNotFound(err) {
			response.WriteHeader(http.StatusNotFound)
			WriteResponse(logger, response, fmt.Sprintf("ElementalHost '%s' not found", hostName))
		} else {
			logger.Error(err, "Could not fetch ElementalHost")
			response.WriteHeader(http.StatusInternalServerError)
			WriteResponse(logger, response, fmt.Sprintf("Could not fetch ElementalHost '%s'", hostName))
		}
		return
	}

	// Unmarshal PATCH request body
	hostPatchRequest := &HostPatchRequest{}
	if err := json.NewDecoder(request.Body).Decode(hostPatchRequest); err != nil {
		response.WriteHeader(http.StatusBadRequest)
		WriteResponse(logger, response, fmt.Errorf("Could not decode request: %w", err).Error())
		return
	}

	// Validate PATCH request
	if hostPatchRequest.Bootstrapped != nil {
		if *hostPatchRequest.Bootstrapped && host.Spec.BootstrapSecret == nil {
			response.WriteHeader(http.StatusBadRequest)
			WriteResponse(logger, response, "Can't mark the Host as bootstrapped if no bootstrap secret has been associated yet.")
			return
		}
	}

	// Patch the object
	patchHelper, err := patch.NewHelper(host, h.k8sClient)
	if err != nil {
		logger.Error(err, "Initializing ElementalHost patch helper")
		response.WriteHeader(http.StatusInternalServerError)
		WriteResponse(logger, response, "Could not initialize ElementalHost patch helper")
		return
	}

	hostPatchRequest.fromElementalHost(host)
	if err := patchHelper.Patch(request.Context(), host); err != nil {
		logger.Error(err, "Could not patch ElementalHost")
		response.WriteHeader(http.StatusInternalServerError)
		WriteResponse(logger, response, fmt.Sprintf("Could not patch ElementalHost '%s'", hostName))
		return
	}

	// Fetch the updated host
	host = &infrastructurev1beta1.ElementalHost{}
	if err := h.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: namespace, Name: hostName}, host); err != nil {
		if k8sapierrors.IsNotFound(err) {
			response.WriteHeader(http.StatusNotFound)
			WriteResponse(logger, response, fmt.Sprintf("Updated ElementalHost '%s' not found", hostName))
		} else {
			logger.Error(err, "Could not fetch updated ElementalHost")
			response.WriteHeader(http.StatusInternalServerError)
			WriteResponse(logger, response, fmt.Sprintf("Could not fetch updated ElementalHost '%s'", hostName))
		}
		return
	}

	// Serialize response to JSON
	hostResponse := HostResponse{}
	hostResponse.fromElementalHost(*host)
	responseBytes, err := json.Marshal(hostResponse)
	if err != nil {
		h.logger.Error(err, "Could not encode response body", "host", fmt.Sprintf("%+v", hostResponse))
		response.WriteHeader(http.StatusInternalServerError)
		WriteResponse(logger, response, fmt.Errorf("Could not encode response body: %w", err).Error())
		return
	}

	logger.Info("ElementalHost patched successfully")
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	WriteResponseBytes(logger, response, responseBytes)
}

var _ OpenAPIDecoratedHandler = (*PostElementalHostHandler)(nil)
var _ http.Handler = (*PostElementalHostHandler)(nil)

type PostElementalHostHandler struct {
	logger    logr.Logger
	k8sClient client.Client
}

func NewPostElementalHostHandler(logger logr.Logger, k8sClient client.Client) *PostElementalHostHandler {
	return &PostElementalHostHandler{
		logger:    logger,
		k8sClient: k8sClient,
	}
}

func (h *PostElementalHostHandler) SetupOpenAPIOperation(oc openapi.OperationContext) error {
	oc.SetSummary("Create a new ElementalHost")
	oc.SetDescription("This endpoint create a new ElementalHost.")

	oc.AddReqStructure(HostCreateRequest{})

	oc.AddRespStructure(nil, WithDecoration("ElementalHost correctly created. Location Header contains its URI", "", http.StatusCreated))
	oc.AddRespStructure(nil, WithDecoration("ElementalHost with same name within this ElementalRegistration already exists", "text/html", http.StatusConflict))
	oc.AddRespStructure(nil, WithDecoration("ElementalRegistration not found", "text/html", http.StatusNotFound))
	oc.AddRespStructure(nil, WithDecoration("ElementalHost request is badly formatted", "text/html", http.StatusBadRequest))
	oc.AddRespStructure(nil, WithDecoration("", "text/html", http.StatusInternalServerError))

	return nil
}

func (h *PostElementalHostHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	pathVars := mux.Vars(request)
	namespace := pathVars["namespace"]
	registrationName := pathVars["registrationName"]

	logger := h.logger.WithValues(log.KeyNamespace, namespace).
		WithValues(log.KeyElementalMachineRegistration, registrationName)
	logger.Info("Creating new ElementalHost")

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

	// Unmarshal POST request body.
	logger.V(log.DebugLevel).Info("Unmarshalling request body")
	hostCreateRequest := &HostCreateRequest{}
	if err := json.NewDecoder(request.Body).Decode(hostCreateRequest); err != nil {
		response.WriteHeader(http.StatusBadRequest)
		WriteResponse(logger, response, fmt.Errorf("Could not decode request body: %w", err).Error())
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
	if err := h.k8sClient.Create(request.Context(), &newHost); err != nil {
		if k8sapierrors.IsAlreadyExists(err) {
			response.WriteHeader(http.StatusConflict)
			WriteResponse(logger, response, fmt.Sprintf("Host '%s' in namespace '%s' already exists", namespace, newHost.Name))
		} else {
			logger.Error(err, "Could not create ElementalHost", "namespace", namespace, "hostName", newHost.Name)
			response.WriteHeader(http.StatusInternalServerError)
			WriteResponse(logger, response, fmt.Sprintf("Could not create Elemental Host '%s'", newHost.Name))
		}
		return
	}

	logger.Info("ElementalHost created successfully", log.KeyElementalHost, newHost.Name)

	response.Header().Set("Location", fmt.Sprintf("%s%s/namespaces/%s/registrations/%s/hosts/%s", PrefixAPI, PrefixV1, namespace, registrationName, newHost.Name))
	response.WriteHeader(http.StatusCreated)
}

var _ OpenAPIDecoratedHandler = (*GetElementalHostBootstrapHandler)(nil)
var _ http.Handler = (*GetElementalHostBootstrapHandler)(nil)

type GetElementalHostBootstrapHandler struct {
	logger    logr.Logger
	k8sClient client.Client
}

func NewGetElementalHostBootstrapHandler(logger logr.Logger, k8sClient client.Client) *GetElementalHostBootstrapHandler {
	return &GetElementalHostBootstrapHandler{
		logger:    logger,
		k8sClient: k8sClient,
	}
}

func (h *GetElementalHostBootstrapHandler) SetupOpenAPIOperation(oc openapi.OperationContext) error {
	oc.SetSummary("Get ElementalHost bootstrap")
	oc.SetDescription("This endpoint returns the ElementalHost bootstrap instructions.")

	oc.AddReqStructure(BootstrapGetRequest{})

	oc.AddRespStructure(BootstrapResponse{}, WithDecoration("Returns the ElementalHost bootstrap instructions", "application/json", http.StatusOK))
	oc.AddRespStructure(nil, WithDecoration("If the ElementalRegistration or ElementalHost are not found, or if there are no bootstrap instructions yet", "text/html", http.StatusNotFound))
	oc.AddRespStructure(nil, WithDecoration("", "text/html", http.StatusInternalServerError))

	return nil
}

func (h *GetElementalHostBootstrapHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	pathVars := mux.Vars(request)
	namespace := pathVars["namespace"]
	registrationName := pathVars["registrationName"]
	hostName := pathVars["hostName"]

	logger := h.logger.WithValues(log.KeyNamespace, namespace).
		WithValues(log.KeyElementalMachineRegistration, registrationName).
		WithValues(log.KeyElementalHost, hostName)
	logger.Info("Getting ElementalHost Bootstrap")

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

	// Fetch host
	host := &infrastructurev1beta1.ElementalHost{}
	if err := h.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: namespace, Name: hostName}, host); err != nil {
		if k8sapierrors.IsNotFound(err) {
			response.WriteHeader(http.StatusNotFound)
			WriteResponse(logger, response, fmt.Sprintf("ElementalHost '%s' not found", hostName))
		} else {
			logger.Error(err, "Could not fetch ElementalHost")
			response.WriteHeader(http.StatusInternalServerError)
			WriteResponse(logger, response, fmt.Sprintf("Could not fetch ElementalHost '%s'", hostName))
		}
		return
	}

	// Check if there is any Bootstrap secret associated to this host
	if host.Spec.BootstrapSecret == nil {
		response.WriteHeader(http.StatusNotFound)
		WriteResponse(logger, response, "There is no associated boostrap secret yet")
		return
	}

	// Fetch bootstrap secret
	bootstrapSecret := &corev1.Secret{}
	if err := h.k8sClient.Get(request.Context(), k8sclient.ObjectKey{Namespace: host.Spec.BootstrapSecret.Namespace, Name: host.Spec.BootstrapSecret.Name}, bootstrapSecret); err != nil {
		if k8sapierrors.IsNotFound(err) {
			logger.Error(err, "Could not find expected Bootstrap secret", log.KeyBootstrapSecret, host.Spec.BootstrapSecret.Name)
			response.WriteHeader(http.StatusInternalServerError)
			WriteResponse(logger, response, "Could not find expected Bootstrap secret")
		} else {
			logger.Error(err, "Could not fetch Bootstrap secret", log.KeyBootstrapSecret, host.Spec.BootstrapSecret.Name)
			response.WriteHeader(http.StatusInternalServerError)
			WriteResponse(logger, response, "Could not fetch Bootstrap secret")
		}
		return
	}

	// Encode response
	bootstrapResponse := &BootstrapResponse{}
	if err := bootstrapResponse.fromSecret(bootstrapSecret); err != nil {
		logger.Error(err, "Could not prepare bootstrap response")
		response.WriteHeader(http.StatusInternalServerError)
		WriteResponse(logger, response, fmt.Errorf("Could not prepare bootstrap response: %w", err).Error())
	}

	responseBytes, err := json.Marshal(bootstrapResponse)
	if err != nil {
		logger.Error(err, "Could not encode bootstrap response body")
		response.WriteHeader(http.StatusInternalServerError)
		WriteResponse(logger, response, fmt.Errorf("Could not encode bootstrap response body: %w", err).Error())
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	WriteResponseBytes(logger, response, responseBytes)
}
