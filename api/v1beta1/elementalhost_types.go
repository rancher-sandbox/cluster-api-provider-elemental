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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ElementalHostSpec defines the desired state of ElementalHost.
type ElementalHostSpec struct {
	// BootstrapSecret is an optional reference to a Cluster API Secret
	// for bootstrap purpose.
	// +optional
	BootstrapSecret *corev1.ObjectReference `json:"bootstrapSecret,omitempty"`
	// MachineRef is an optional reference to a Cluster API ElementalMachine
	// using this host.
	// +optional
	MachineRef *corev1.ObjectReference `json:"machineRef,omitempty"`
	// PubKey is the host public key to verify when authenticating
	// Elemental API requests for this host.
	PubKey string `json:"pubKey,omitempty"`
}

// ElementalHostStatus defines the observed state of ElementalHost.
type ElementalHostStatus struct {
	// Conditions defines current service state of the ElementalHost.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (h *ElementalHost) GetConditions() clusterv1.Conditions {
	return h.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (h *ElementalHost) SetConditions(conditions clusterv1.Conditions) {
	h.Status.Conditions = conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster"
//+kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.labels['elementalhost\\.infrastructure\\.cluster\\.x-k8s\\.io/machine-name']",description="Machine object associated to this ElementalHost (through ElementalMachine)"
//+kubebuilder:printcolumn:name="ElementalMachine",type="string",JSONPath=".spec.machineRef[?(@.kind==\"ElementalMachine\")].name",description="ElementalMachine object associated to this ElementalHost"
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="ElementalHost ready status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of ElementalHost"

// ElementalHost is the Schema for the elementalhosts API.
type ElementalHost struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElementalHostSpec   `json:"spec,omitempty"`
	Status ElementalHostStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ElementalHostList contains a list of ElementalHost.
type ElementalHostList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElementalHost `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElementalHost{}, &ElementalHostList{})
}
