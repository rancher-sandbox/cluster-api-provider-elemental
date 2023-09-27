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
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// ElementalMachineRegistrationSpec defines the desired state of ElementalMachineRegistration.
type ElementalMachineRegistrationSpec struct {
	// MachineLabels are labels propagated to each ElementalHost object linked to this registration.
	// +optional
	MachineLabels map[string]string `json:"machineLabels,omitempty"`
	// MachineAnnotations are labels propagated to each ElementalHost object linked to this registration.
	// +optional
	MachineAnnotations map[string]string `json:"machineAnnotations,omitempty"`
	// Config points to Elemental machine configuration.
	// +optional
	Config *Config `json:"config,omitempty"`
}

// ElementalMachineRegistrationStatus defines the observed state of ElementalMachineRegistration.
type ElementalMachineRegistrationStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ElementalMachineRegistration is the Schema for the elementalmachineregistrations API.
type ElementalMachineRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElementalMachineRegistrationSpec   `json:"spec,omitempty"`
	Status ElementalMachineRegistrationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ElementalMachineRegistrationList contains a list of ElementalMachineRegistration.
type ElementalMachineRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElementalMachineRegistration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElementalMachineRegistration{}, &ElementalMachineRegistrationList{})
}

type Config struct {
	// +optional
	Elemental Elemental `json:"elemental,omitempty" yaml:"elemental"`
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	CloudConfig map[string]runtime.RawExtension `json:"cloudConfig,omitempty" yaml:"cloudConfig,omitempty"`
}

type Install struct {
	// +optional
	Firmware string `json:"firmware,omitempty" yaml:"firmware,omitempty"`
	// +optional
	Device string `json:"device,omitempty" yaml:"device,omitempty"`
	// +optional
	NoFormat bool `json:"noFormat,omitempty" yaml:"noFormat,omitempty"`
	// +optional
	ConfigURLs []string `json:"configUrls,omitempty" yaml:"configUrls,omitempty"`
	// +optional
	ISO string `json:"iso,omitempty" yaml:"iso,omitempty"`
	// +optional
	SystemURI string `json:"systemUri,omitempty" yaml:"systemUri,omitempty"`
	// +optional
	Debug bool `json:"debug,omitempty" yaml:"debug,omitempty"`
	// +optional
	TTY string `json:"tty,omitempty" yaml:"tty,omitempty"`
	// +optional
	PowerOff bool `json:"poweroff,omitempty" yaml:"poweroff,omitempty"`
	// +optional
	Reboot bool `json:"reboot,omitempty" yaml:"reboot,omitempty"`
	// +optional
	EjectCD bool `json:"ejectCd,omitempty" yaml:"ejectCd,omitempty"`
	// +optional
	DisableBootEntry bool `json:"disableBootEntry,omitempty" yaml:"disableBootEntry,omitempty"`
	// +optional
	ConfigDir string `json:"configDir,omitempty" yaml:"configDir,omitempty"`
}

type Reset struct {
	// +optional
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty" mapstructure:"enabled"`
	// +optional
	// +kubebuilder:default:=true
	ResetPersistent bool `json:"resetPersistent,omitempty" yaml:"resetPersistent,omitempty" mapstructure:"resetPersistent"`
	// +optional
	// +kubebuilder:default:=true
	ResetOEM bool `json:"resetOem,omitempty" yaml:"resetOem,omitempty" mapstructure:"resetOem"`
	// +optional
	ConfigURLs []string `json:"configUrls,omitempty" yaml:"configUrls,omitempty" mapstructure:"configUrls"`
	// +optional
	SystemURI string `json:"systemUri,omitempty" yaml:"systemUri,omitempty" mapstructure:"systemUri"`
	// +optional
	Debug bool `json:"debug,omitempty" yaml:"debug,omitempty" mapstructure:"debug"`
	// +optional
	PowerOff bool `json:"poweroff,omitempty" yaml:"poweroff,omitempty" mapstructure:"poweroff"`
	// +optional
	// +kubebuilder:default:=true
	Reboot bool `json:"reboot,omitempty" yaml:"reboot,omitempty" mapstructure:"reboot"`
}

type Registration struct {
	// +optional
	URL string `json:"url,omitempty" yaml:"url,omitempty" mapstructure:"url"`
	// +optional
	CACert string `json:"caCert,omitempty" yaml:"caCert,omitempty" mapstructure:"caCert"`
	// +optional
	NoSMBIOS bool `json:"noSmbios,omitempty" yaml:"noSmbios,omitempty" mapstructure:"noSmbios"`
	// +optional
	Hostname Hostname `json:"hostname,omitempty" yaml:"hostname,omitempty" mapstructure:"hostname"`
}

type Hostname struct {
	// +optional
	UseExisting bool `json:"useExisting,omitempty" yaml:"useExisting,omitempty" mapstructure:"useExisting"`
	// +optional
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty" mapstructure:"prefix"`
}

type Elemental struct {
	// +optional
	Install Install `json:"install,omitempty" yaml:"install,omitempty"`
	// +optional
	// +kubebuilder:default:={"resetPersistent":true,"resetOem":true,"reboot":true}
	Reset Reset `json:"reset,omitempty" yaml:"reset,omitempty"`
	// +optional
	Registration Registration `json:"registration,omitempty" yaml:"registration,omitempty"`
}