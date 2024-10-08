/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ElementalRegistrationSpec defines the desired state of ElementalRegistration.
type ElementalRegistrationSpec struct {
	// HostLabels are labels propagated to each ElementalHost object linked to this registration.
	// +optional
	HostLabels map[string]string `json:"hostLabels,omitempty"`
	// HostAnnotations are labels propagated to each ElementalHost object linked to this registration.
	// +optional
	HostAnnotations map[string]string `json:"hostAnnotations,omitempty"`
	// Config points to Elemental machine configuration.
	// +optional
	Config Config `json:"config,omitempty"`
	// PrivateKeyRef is a reference to a secret containing the private key used to generate registration tokens
	PrivateKeyRef *corev1.ObjectReference `json:"privateKeyRef,omitempty"`
}

// ElementalRegistrationStatus defines the observed state of ElementalRegistration.
type ElementalRegistrationStatus struct {
	// Conditions defines current service state of the ElementalRegistration.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (m *ElementalRegistration) GetConditions() clusterv1.Conditions {
	return m.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (m *ElementalRegistration) SetConditions(conditions clusterv1.Conditions) {
	m.Status.Conditions = conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ElementalRegistration is the Schema for the ElementalRegistrations API.
type ElementalRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElementalRegistrationSpec   `json:"spec,omitempty"`
	Status ElementalRegistrationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ElementalRegistrationList contains a list of ElementalRegistration.
type ElementalRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElementalRegistration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElementalRegistration{}, &ElementalRegistrationList{})
}

type Config struct {
	// +optional
	Elemental Elemental `json:"elemental,omitempty" yaml:"elemental"`
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	CloudConfig map[string]runtime.RawExtension `json:"cloudConfig,omitempty" yaml:"cloudConfig,omitempty"`
}

type Registration struct {
	// +optional
	URI string `json:"uri,omitempty" yaml:"uri,omitempty" mapstructure:"uri"`
	// +optional
	CACert string `json:"caCert,omitempty" yaml:"caCert,omitempty" mapstructure:"caCert"`
	// +optional
	TokenDuration time.Duration `json:"tokenDuration,omitempty" yaml:"tokenDuration,omitempty" mapstructure:"tokenDuration"`
	// +optional
	Token string `json:"token,omitempty" yaml:"token,omitempty" mapstructure:"token"`
}

type Agent struct {
	// +optional
	// +kubebuilder:default:="/var/lib/elemental/agent"
	WorkDir string `json:"workDir,omitempty" yaml:"workDir,omitempty" mapstructure:"workDir"`
	// +optional
	// +kubebuilder:default:={"useExisting":true}
	Hostname Hostname `json:"hostname,omitempty" yaml:"hostname,omitempty" mapstructure:"hostname"`
	// +optional
	NoSMBIOS bool `json:"noSmbios,omitempty" yaml:"noSmbios,omitempty" mapstructure:"noSmbios"`
	// +optional
	Debug bool `json:"debug,omitempty" yaml:"debug,omitempty" mapstructure:"debug"`
	// +optional
	// +kubebuilder:default:="/usr/lib/elemental/plugins/elemental.so"
	OSPlugin string `json:"osPlugin,omitempty" yaml:"osPlugin,omitempty" mapstructure:"osPlugin"`
	// +optional
	// +kubebuilder:default:=10000000000
	Reconciliation time.Duration `json:"reconciliation,omitempty" yaml:"reconciliation,omitempty" mapstructure:"reconciliation"`
	// +optional
	InsecureAllowHTTP bool `json:"insecureAllowHttp,omitempty" yaml:"insecureAllowHttp,omitempty" mapstructure:"insecureAllowHttp"`
	// +optional
	InsecureSkipTLSVerify bool `json:"insecureSkipTlsVerify,omitempty" yaml:"insecureSkipTlsVerify,omitempty" mapstructure:"insecureSkipTlsVerify"`
	// +optional
	UseSystemCertPool bool `json:"useSystemCertPool,omitempty" yaml:"useSystemCertPool,omitempty" mapstructure:"useSystemCertPool"`
	// +optional
	PostInstall PostAction `json:"postInstall,omitempty" yaml:"postInstall,omitempty" mapstructure:"postInstall"`
	// +optional
	PostReset PostAction `json:"postReset,omitempty" yaml:"postReset,omitempty" mapstructure:"postReset"`
}

// PostAction is used to return instructions to the cli after a Phase is handled.
type PostAction struct {
	// +optional
	PowerOff bool `json:"powerOff,omitempty" yaml:"powerOff,omitempty" mapstructure:"powerOff"`
	// +optional
	Reboot bool `json:"reboot,omitempty" yaml:"reboot,omitempty" mapstructure:"reboot"`
}

type Hostname struct {
	// +optional
	// +kubebuilder:default:=false
	UseExisting bool `json:"useExisting,omitempty" yaml:"useExisting,omitempty" mapstructure:"useExisting"`
	// +optional
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty" mapstructure:"prefix"`
}

type Elemental struct {
	// +optional
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:XPreserveUnknownFields
	Install map[string]runtime.RawExtension `json:"install,omitempty" yaml:"install,omitempty"`
	// +optional
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:XPreserveUnknownFields
	Reset map[string]runtime.RawExtension `json:"reset,omitempty" yaml:"reset,omitempty"`
	// +optional
	Registration Registration `json:"registration,omitempty" yaml:"registration,omitempty"`
	// +optional
	// +kubebuilder:default:={"debug":false,"reconciliation":10000000000,"hostname":{"useExisting":false},"osPlugin":"/usr/lib/elemental/plugins/elemental.so"}
	Agent Agent `json:"agent,omitempty" yaml:"agent,omitempty"`
}
