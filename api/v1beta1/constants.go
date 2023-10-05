package v1beta1

import "k8s.io/apimachinery/pkg/runtime/schema"

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
	AnnotationElementalHostPublicKey         = "elementalhost.infrastructure.cluster.x-k8s.io/pubkey"
)
