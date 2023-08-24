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

package v1beta3

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ElementalMachineRegistrationSpec defines the desired state of ElementalMachineRegistration
type ElementalMachineRegistrationSpec struct {
	// BootstrapTokenRef is a reference to the object containing a Kubernetes token.
	// This token will be used by ElementalHosts to perform the initial registration.
	// +optional
	BootstrapTokenRef *corev1.ObjectReference `json:"bootstrapTokenRef"`
}

// ElementalMachineRegistrationStatus defines the observed state of ElementalMachineRegistration
type ElementalMachineRegistrationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ElementalMachineRegistration is the Schema for the elementalmachineregistrations API
type ElementalMachineRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElementalMachineRegistrationSpec   `json:"spec,omitempty"`
	Status ElementalMachineRegistrationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ElementalMachineRegistrationList contains a list of ElementalMachineRegistration
type ElementalMachineRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElementalMachineRegistration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElementalMachineRegistration{}, &ElementalMachineRegistrationList{})
}
