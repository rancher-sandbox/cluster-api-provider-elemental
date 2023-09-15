package log

// Logging levels.
const (
	DebugLevel = 1
	InfoLevel  = 0
)

// Structured logging Keys.
const (
	// The namespace name that the resource belongs to.
	KeyNamespace = "Namespace"
	// The ElementalMachineRegistration name.
	KeyElementalMachineRegistration = "ElementalMachineRegistration"
	// The ElementalCluster name.
	KeyElementalCluster = "ElementalCluster"
	// The CAPI Cluster name.
	KeyCluster = "Cluster"
	// The ElementalMachine name.
	KeyElementalMachine = "ElementalMachine"
	// The CAPI Machine name.
	KeyMachine = "Machine"
	// The ElementalHost name.
	KeyElementalHost = "ElementalHost"
	// The Bootstrap Secret name.
	KeyBootstrapSecret = "BootstrapSecret"
)
