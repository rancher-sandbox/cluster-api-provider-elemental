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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ElementalHostSpec defines the desired state of ElementalHost
type ElementalHostSpec struct {
	// BootstrapSecret is an optional reference to a Cluster API Secret
	// for bootstrap purpose
	// +optional
	BootstrapSecret *corev1.ObjectReference `json:"bootstrapSecret,omitempty"`
}

// ElementalHostStatus defines the observed state of ElementalHost
type ElementalHostStatus struct {
	// MachineRef is an optional reference to a Cluster API ElementalMachine
	// using this host.
	// +optional
	MachineRef *corev1.ObjectReference `json:"machineRef,omitempty"`
	// Installed references this host to have been successfully installed by Elemental
	// +optional
	Installed bool `json:"installed,omitempty"`
	// Bootstrapped references this host to have applied the Boostrap instructions successfully
	// +optional
	Bootstrapped bool `json:"bootstrapped,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ElementalHost is the Schema for the elementalhosts API
type ElementalHost struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElementalHostSpec   `json:"spec,omitempty"`
	Status ElementalHostStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ElementalHostList contains a list of ElementalHost
type ElementalHostList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElementalHost `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElementalHost{}, &ElementalHostList{})
}
