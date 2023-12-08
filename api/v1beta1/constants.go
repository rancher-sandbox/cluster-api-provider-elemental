package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// API Info.
const (
	InfraGroup   = "infrastructure.cluster.x-k8s.io"
	InfraVersion = "v1beta1"
)

var (
	InfraGroupVersion = schema.GroupVersion{Group: InfraGroup, Version: InfraVersion}
)

// Finalizers.
const (
	FinalizerElementalMachine = "elementalmachine.infrastructure.cluster.x-k8s.io"
)

// Annotations.
const (
	AnnotationElementalRegistrationName      = "elementalregistration.infrastructure.cluster.x-k8s.io/name"
	AnnotationElementalRegistrationNamespace = "elementalregistration.infrastructure.cluster.x-k8s.io/namespace"
	AnnotationElementalHostPublicKey         = "elementalhost.infrastructure.cluster.x-k8s.io/pub-key"
)

// Labels.
const (
	LabelElementalHostMachineName  = "elementalhost.infrastructure.cluster.x-k8s.io/machine-name"
	LabelElementalHostInstalled    = "elementalhost.infrastructure.cluster.x-k8s.io/installed"
	LabelElementalHostBootstrapped = "elementalhost.infrastructure.cluster.x-k8s.io/bootstrapped"
	LabelElementalHostNeedsReset   = "elementalhost.infrastructure.cluster.x-k8s.io/needs-reset"
	LabelElementalHostReset        = "elementalhost.infrastructure.cluster.x-k8s.io/reset"
)

// Conditions.
// See: https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20200506-conditions.md

// ElementalHost Conditions and Reasons.
const (
	// RegistrationReady describes the Host registration phase.
	RegistrationReady clusterv1.ConditionType = "RegistrationReady"
	// RegistrationFailedReason indicates a failure within the registration process.
	// Since the ElementalHost creation starts this process, this reason most likely indicates
	// a post-registration failure, for example if the elemental-agent was unable to install
	// its identity file into the just registered host.
	RegistrationFailedReason = "RegistrationFailed"

	// InstallationReady describes the Host installation phase.
	InstallationReady clusterv1.ConditionType = "InstallationReady"
	// WaitingForInstallationReason indicates that this Host was registered but no installation has taken place yet.
	WaitingForInstallationReason                                     = "WaitingForInstallation"
	WaitingForInstallationReasonSeverity clusterv1.ConditionSeverity = clusterv1.ConditionSeverityInfo
	// CloudConfigInstallationFailedReason indicates a failure when applying the registration cloud-config to the host.
	CloudConfigInstallationFailedReason = "CloudConfigInstallationFailed"
	// InstallationFailedReason indicates a failure within the installation process.
	InstallationFailedReason = "InstallationFailed"

	// BootstrapReady describes the Host bootstrapping phase.
	BootstrapReady clusterv1.ConditionType = "BootstrapReady"
	// WaitingForBootstrapReason indicates that the bootstrap was applied.
	WaitingForBootstrapReason                                     = "WaitingForBootstrap"
	WaitingForBootstrapReasonSeverity clusterv1.ConditionSeverity = clusterv1.ConditionSeverityInfo
	// BootstrapFailedReason indicates a failure with bootstrapping the host.
	BootstrapFailedReason = "BootstrapFailed"

	// ResetReady describes the Host reset phase.
	ResetReady clusterv1.ConditionType = "ResetReady"
	// WaitingForResetReason indicates that the Host reset was triggered.
	WaitingForResetReason                                     = "WaitingForReset"
	WaitingForResetReasonSeverity clusterv1.ConditionSeverity = clusterv1.ConditionSeverityInfo
	// ResetFailedReason indicates that the Host reset failed.
	ResetFailedReason = "ResetFailed"
)

// ElementalMachine Conditions and Reasons.
const (
	// AssociationReady describes the ElementalMachine to ElementalHost association status.
	AssociationReady clusterv1.ConditionType = "AssociationReady"
	// MissingMachineOwnerReason indicates the ElementalMachine is not owner by any CAPI Machine.
	MissingMachineOwnerReason = "MissingMachineOwner"
	// MissingAssociatedClusterReason indicates the ElementalMachine is not part of any CAPI Cluster.
	MissingAssociatedClusterReason = "MissingAssociatedCluster"
	// MissingClusterInfrastructureReadyReason indicates the CAPI Cluster Status.InfrastructureReady is false.
	MissingClusterInfrastructureReadyReason = "MissingClusterInfrastructureReady"
	// MissingBootstrapSecretReason indicates that no bootstrap secret has been found.
	MissingBootstrapSecretReason = "MissingBootstrapSecret"
	// MissingAvailableHostsReason indicates that no ElementalHost is available for association.
	MissingAvailableHostsReason = "MissingAvailableHosts"
	// AssociatedHostNotFoundReason indicates that a previously associated ElementalHost is not found.
	// This can be the consequence of deleting an existing ElementalHost, for example to replace defective hardware.
	// This Reason should be transient as the provider should try to associate the ElementalMachine with a new available ElementalHost.
	AssociatedHostNotFoundReason                                     = "AssociatedHostNotFound"
	AssociatedHostNotFoundReasonSeverity clusterv1.ConditionSeverity = clusterv1.ConditionSeverityWarning

	// HostReady summarizes the status of the associated ElementalHost.
	HostReady clusterv1.ConditionType = "HostReady"
	// HostWaitingForBootstrapReason indicates that the bootstrap was applied on the host
	// and the provider is waiting for success confirmation.
	HostWaitingForBootstrapReason                                     = "HostWaitingForBootstrap"
	HostWaitingForBootstrapReasonSeverity clusterv1.ConditionSeverity = clusterv1.ConditionSeverityInfo
	// HostBoostrapFailedReason indicates that the underlying host failed to apply the CAPI bootstrap.
	HostBoostrapFailedReason = "HostBootstrapFailed"
)

// ElementalCluster Conditions and Reasons.
const (
	// ControlPlaneEndpointReady describes the status of the ControlPlaneEndpoint.
	ControlPlaneEndpointReady clusterv1.ConditionType = "ControlPlaneEndpointReady"
	// MissingControlPlaneEndpointReason indicates that the ElementalCluster.spec.controlPlaneEndpoint was not defined.
	MissingControlPlaneEndpointReason = "MissingControlPlaneEndpoint"
)
