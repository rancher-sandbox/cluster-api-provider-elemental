package api

import (
	"errors"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"sigs.k8s.io/cluster-api/util/conditions"
)

var ErrBootstrapSecretNoConfig = errors.New("CAPI bootstrap secret does not contain any config")

type HostCreateRequest struct {
	Auth    string `header:"Authorization"`
	RegAuth string `header:"Registration-Authorization"`

	Namespace        string `path:"namespace"`
	RegistrationName string `path:"registrationName"`

	Name        string            `json:"name,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	PubKey      string            `json:"pubKey,omitempty"`
}

func (h *HostCreateRequest) toElementalHost(namespace string) infrastructurev1beta1.ElementalHost {
	return infrastructurev1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:        h.Name,
			Namespace:   namespace,
			Labels:      h.Labels,
			Annotations: h.Annotations,
		},
		Spec: infrastructurev1beta1.ElementalHostSpec{
			PubKey: h.PubKey,
		},
	}
}

type HostDeleteRequest struct {
	Auth string `header:"Authorization"`

	Namespace        string `path:"namespace"`
	RegistrationName string `path:"registrationName"`
	HostName         string `path:"hostName"`
}

type HostPatchRequest struct {
	Auth string `header:"Authorization"`

	Namespace        string `path:"namespace"`
	RegistrationName string `path:"registrationName"`
	HostName         string `path:"hostName"`

	Annotations  map[string]string `json:"annotations,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Bootstrapped *bool             `json:"bootstrapped,omitempty"`
	Installed    *bool             `json:"installed,omitempty"`
	Reset        *bool             `json:"reset,omitempty"`

	Condition *clusterv1.Condition             `json:"condition,omitempty"`
	Phase     *infrastructurev1beta1.HostPhase `json:"phase,omitempty"`
}

func (h *HostPatchRequest) SetCondition(conditionType clusterv1.ConditionType, status corev1.ConditionStatus, severity clusterv1.ConditionSeverity, reason string, message string) {
	h.Condition = &clusterv1.Condition{
		Type:     conditionType,
		Status:   status,
		Severity: severity,
		Reason:   reason,
		Message:  message,
	}
}

func (h *HostPatchRequest) applyToElementalHost(elementalHost *infrastructurev1beta1.ElementalHost) {
	if elementalHost.Annotations == nil {
		elementalHost.Annotations = map[string]string{}
	}
	if elementalHost.Labels == nil {
		elementalHost.Labels = map[string]string{}
	}
	maps.Copy(elementalHost.Annotations, h.Annotations)
	maps.Copy(elementalHost.Labels, h.Labels)
	// Map request values to ElementalHost labels
	if h.Installed != nil {
		elementalHost.Labels[infrastructurev1beta1.LabelElementalHostInstalled] = "true"
	}
	if h.Bootstrapped != nil {
		elementalHost.Labels[infrastructurev1beta1.LabelElementalHostBootstrapped] = "true"
	}
	if h.Reset != nil {
		elementalHost.Labels[infrastructurev1beta1.LabelElementalHostReset] = "true"
	}
	if elementalHost.Status.Conditions == nil {
		elementalHost.Status.Conditions = clusterv1.Conditions{}
	}
	// Set the patch condition to the ElementalHost object.
	conditions.Set(elementalHost, h.Condition)
	// Always update the Summary after conditions change
	conditions.SetSummary(elementalHost)

	if h.Phase != nil {
		elementalHost.Status.Phase = *h.Phase
	}
}

type HostResponse struct {
	Name           string            `json:"name,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	BootstrapReady bool              `json:"bootstrapReady,omitempty"`
	Bootstrapped   bool              `json:"bootstrapped,omitempty"`
	Installed      bool              `json:"installed,omitempty"`
	NeedsReset     bool              `json:"needsReset,omitempty"`
}

func (h *HostResponse) fromElementalHost(elementalHost infrastructurev1beta1.ElementalHost) {
	h.Name = elementalHost.Name
	h.Annotations = elementalHost.Annotations
	h.Labels = elementalHost.Labels
	h.BootstrapReady = elementalHost.Spec.BootstrapSecret != nil
	if elementalHost.Labels == nil {
		return
	}
	// Map ElementalHost labels to response values
	if value, found := elementalHost.Labels[infrastructurev1beta1.LabelElementalHostBootstrapped]; found && value == "true" {
		h.Bootstrapped = true
	}
	if value, found := elementalHost.Labels[infrastructurev1beta1.LabelElementalHostInstalled]; found && value == "true" {
		h.Installed = true
	}
	if value, found := elementalHost.Labels[infrastructurev1beta1.LabelElementalHostNeedsReset]; found && value == "true" {
		h.NeedsReset = true
	}
}

type RegistrationGetRequest struct {
	RegAuth string `header:"Registration-Authorization"`

	Namespace        string `path:"namespace"`
	RegistrationName string `path:"registrationName"`
}

type RegistrationResponse struct {
	// HostLabels are labels propagated to each ElementalHost object linked to this registration.
	// +optional
	HostLabels map[string]string `json:"hostLabels,omitempty"`
	// HostAnnotations are labels propagated to each ElementalHost object linked to this registration.
	// +optional
	HostAnnotations map[string]string `json:"hostAnnotations,omitempty"`
	// Config points to Elemental machine configuration.
	// +optional
	Config infrastructurev1beta1.Config `json:"config,omitempty"`
}

func (r *RegistrationResponse) fromElementalRegistration(elementalRegistration infrastructurev1beta1.ElementalRegistration) {
	r.HostLabels = elementalRegistration.Spec.HostLabels
	r.HostAnnotations = elementalRegistration.Spec.HostAnnotations
	r.Config = elementalRegistration.Spec.Config
}

type BootstrapGetRequest struct {
	Auth string `header:"Authorization"`

	Namespace        string `path:"namespace"`
	RegistrationName string `path:"registrationName"`
	HostName         string `path:"hostName"`
}

type BootstrapResponse struct {
	Format string `json:"format"`
	Config string `json:"config"`
}

func (b *BootstrapResponse) fromSecret(secret *corev1.Secret) error {
	b.Format = "cloud-config" // Assume 'cloud-config' by default.
	if format, found := secret.Data["format"]; found {
		b.Format = string(format)
	}
	if config, found := secret.Data["value"]; found {
		b.Config = string(config)
		return nil
	}
	return ErrBootstrapSecretNoConfig
}
