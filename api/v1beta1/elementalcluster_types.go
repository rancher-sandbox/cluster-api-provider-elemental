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

// ElementalClusterSpec defines the desired state of ElementalCluster.
type ElementalClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`
}

// ElementalClusterStatus defines the observed state of ElementalCluster.
type ElementalClusterStatus struct {
	// +kubebuilder:default=false
	// Ready indicates the provider-specific infrastructure has been provisioned and is ready.
	Ready bool `json:"ready,omitempty"`

	// Conditions defines current service state of the ElementalCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// FailureDomains defines the failure domains that machines should be placed in.
	// +optional
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (c *ElementalCluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *ElementalCluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ElementalCluster is the Schema for the elementalclusters API.
type ElementalCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElementalClusterSpec   `json:"spec,omitempty"`
	Status ElementalClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ElementalClusterList contains a list of ElementalCluster.
type ElementalClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElementalCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElementalCluster{}, &ElementalClusterList{})
}
