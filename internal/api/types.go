package api

import (
	"fmt"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HostCreateRequest struct {
	Namespace        string `path:"namespace"`
	RegistrationName string `path:"registrationName"`

	Name        string            `json:"name,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

func (h *HostCreateRequest) toElementalHost(namespace string) infrastructurev1beta1.ElementalHost {
	return infrastructurev1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:        h.Name,
			Namespace:   namespace,
			Labels:      h.Labels,
			Annotations: h.Annotations,
		},
	}
}

type HostDeleteRequest struct {
	Namespace        string `path:"namespace"`
	RegistrationName string `path:"registrationName"`
	HostName         string `path:"hostName"`
}

type HostPatchRequest struct {
	Namespace        string `path:"namespace"`
	RegistrationName string `path:"registrationName"`
	HostName         string `path:"hostName"`

	Annotations  map[string]string `json:"annotations,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Bootstrapped *bool             `json:"bootstrapped,omitempty"`
	Installed    *bool             `json:"installed,omitempty"`
	Reset        *bool             `json:"reset,omitempty"`
}

func (h *HostPatchRequest) applyToElementalHost(elementalHost *infrastructurev1beta1.ElementalHost) {
	elementalHost.Annotations = h.Annotations
	elementalHost.Labels = h.Labels
	if elementalHost.Labels == nil {
		elementalHost.Labels = map[string]string{}
	}
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
	Namespace        string `path:"namespace"`
	RegistrationName string `path:"registrationName"`
	HostName         string `path:"hostName"`
}

type BootstrapResponse struct {
	Files    []WriteFile `json:"write_files" yaml:"write_files"` //nolint:tagliatelle //Matching cloud-init schema
	Commands []string    `json:"runcmd" yaml:"runcmd"`
}

type WriteFile struct {
	Path        string `json:"path" yaml:"path"`
	Owner       string `json:"owner" yaml:"owner"`
	Permissions string `json:"permissions" yaml:"permissions"`
	Content     string `json:"content" yaml:"content"`
}

func (b *BootstrapResponse) fromSecret(secret *corev1.Secret) error {
	data := secret.Data["value"]
	if err := yaml.Unmarshal(data, b); err != nil {
		return fmt.Errorf("unmarshalling bootstrap secret value: %w", err)
	}
	return nil
}
