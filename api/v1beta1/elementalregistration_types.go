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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
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
}

// ElementalRegistrationStatus defines the observed state of ElementalRegistration.
type ElementalRegistrationStatus struct {
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
	URI string `json:"uri,omitempty" yaml:"uri,omitempty" mapstructure:"uri"`
	// +optional
	CACert string `json:"caCert,omitempty" yaml:"caCert,omitempty" mapstructure:"caCert"`
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
	// +kubebuilder:default:="unmanaged"
	Installer string `json:"installer,omitempty" yaml:"installer,omitempty" mapstructure:"installer"`
	// +optional
	// +kubebuilder:default:=10000000000
	Reconciliation time.Duration `json:"reconciliation,omitempty" yaml:"reconciliation,omitempty" mapstructure:"reconciliation"`
	// +optional
	InsecureAllowHTTP bool `json:"insecureAllowHttp,omitempty" yaml:"insecureAllowHttp,omitempty" mapstructure:"insecureAllowHttp"`
	// +optional
	InsecureSkipTLSVerify bool `json:"insecureSkipTlsVerify,omitempty" yaml:"insecureSkipTlsVerify,omitempty" mapstructure:"insecureSkipTlsVerify"`
	// +optional
	UseSystemCertPool bool `json:"useSystemCertPool,omitempty" yaml:"useSystemCertPool,omitempty" mapstructure:"useSystemCertPool"`
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
	// +kubebuilder:default:={"debug":false,"device":"/dev/sda","reboot":true}
	Install Install `json:"install,omitempty" yaml:"install,omitempty"`
	// +optional
	// +kubebuilder:default:={"debug":false,"enabled":false,"resetPersistent":true,"resetOem":true,"reboot":true}
	Reset Reset `json:"reset,omitempty" yaml:"reset,omitempty"`
	// +optional
	Registration Registration `json:"registration,omitempty" yaml:"registration,omitempty"`
	// +optional
	// +kubebuilder:default:={"debug":false,"reconciliation":10000000000,"hostname":{"useExisting":false},"installer":"unmanaged"}
	Agent Agent `json:"agent,omitempty" yaml:"agent,omitempty"`
}
