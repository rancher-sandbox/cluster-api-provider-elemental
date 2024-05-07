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

// ElementalMachineSpec defines the desired state of ElementalMachine.
type ElementalMachineSpec struct {
	// ProviderID references the associated ElementalHost.
	// (elemental://{ElementalHost.Namespace}/{ElementalHost.Name})
	// +optional
	ProviderID *string `json:"providerID,omitempty"` //nolint:tagliatelle

	// Selector can be used to associate ElementalHost that match certain labels.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// HostRef is an optional reference to a ElementalHost
	// using this host.
	// +optional
	HostRef *corev1.ObjectReference `json:"hostRef,omitempty"`
}

// ElementalMachineStatus defines the observed state of ElementalMachine.
type ElementalMachineStatus struct {
	// +kubebuilder:default=false
	// Ready indicates the provider-specific infrastructure has been provisioned and is ready.
	Ready bool `json:"ready,omitempty"`

	// Conditions defines current service state of the ElementalMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// FailureDomains defines the failure domains that machines should be placed in.
	// +optional
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (m *ElementalMachine) GetConditions() clusterv1.Conditions {
	return m.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (m *ElementalMachine) SetConditions(conditions clusterv1.Conditions) {
	m.Status.Conditions = conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster"
//+kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this ElementalMachine"
//+kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description="ElementalMachine ready condition"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of ElementalMachine"

// ElementalMachine is the Schema for the elementalmachines API.
type ElementalMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElementalMachineSpec   `json:"spec,omitempty"`
	Status ElementalMachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ElementalMachineList contains a list of ElementalMachine.
type ElementalMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElementalMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElementalMachine{}, &ElementalMachineList{})
}
