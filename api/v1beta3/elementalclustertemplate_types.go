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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ElementalClusterTemplateSpec defines the desired state of ElementalClusterTemplate
type ElementalClusterTemplateSpec struct {
	Template ElementalClusterTemplateResource `json:"template"`
}

type ElementalClusterTemplateResource struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta metav1.ObjectMeta    `json:"metadata,omitempty"`
	Spec       ElementalClusterSpec `json:"spec"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ElementalClusterTemplate is the Schema for the elementalclustertemplates API
type ElementalClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ElementalClusterTemplateSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// ElementalClusterTemplateList contains a list of ElementalClusterTemplate
type ElementalClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElementalClusterTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElementalClusterTemplate{}, &ElementalClusterTemplateList{})
}
