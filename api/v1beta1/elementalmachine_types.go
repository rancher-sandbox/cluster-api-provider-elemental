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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ElementalMachineSpec defines the desired state of ElementalMachine
type ElementalMachineSpec struct {
	// ProviderID references the associated ElementalHost
	// (elemental://{ElementalHost.Namespace}/{ElementalHost.Name})
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Selector can be used to associate ElementalHost that match certain labels
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// ElementalMachineStatus defines the observed state of ElementalMachine
type ElementalMachineStatus struct {
	// +kubebuilder:default=false
	// Ready indicates the provider-specific infrastructure has been provisioned and is ready
	Ready bool `json:"ready,omitempty"`

	// Conditions defines current service state of the ElementalCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// FailureDomains defines the failure domains that machines should be placed in.
	// +optional
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ElementalMachine is the Schema for the elementalmachines API
type ElementalMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElementalMachineSpec   `json:"spec,omitempty"`
	Status ElementalMachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ElementalMachineList contains a list of ElementalMachine
type ElementalMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElementalMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElementalMachine{}, &ElementalMachineList{})
}
