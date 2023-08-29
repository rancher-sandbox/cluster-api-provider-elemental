package v1beta3

import "k8s.io/apimachinery/pkg/runtime/schema"

// API Info
const (
	InfraGroup   = "infrastructure.cluster.x-k8s.io"
	InfraVersion = "v1beta3"
)

var (
	InfraGroupVersion = schema.GroupVersion{Group: InfraGroup, Version: InfraVersion}
)

// Finalizers
const (
	FinalizerElementalMachine = "elementalmachine.infrastructure.cluster.x-k8s.io"
)

// Annotations
const (
	AnnotationElementalMachineRegistrationName      = "elementalmachineregistration.infrastructure.cluster.x-k8s.io/name"
	AnnotationElementalMachineRegistrationNamespace = "elementalmachineregistration.infrastructure.cluster.x-k8s.io/namespace"
	AnnotationElementalHostPublicKey                = "elementalhost.infrastructure.cluster.x-k8s.io/pubkey"
)
