package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	testBootstrapSecretName = "test-secret"
)

// One CAPI Cluster is setup with one CAPI Machine linked to an ElementalMachine, waiting for association with an available ElementalHost.
//
// Cluster <--> Machine <--> ElementalMachine <(this test coverage)> ElementalHost
//
// Several ElementalHosts with different conditions are added in the same namespace to add a bit of chaos.
//
// In order to spot flaky tests, consider running them for a while with `GINKGO_EXTRA_ARGS="--until-it-fails" make test`.
var _ = Describe("ElementalMachine controller association", Label("controller", "elemental-machine"), Ordered, func() {
	ctx := context.Background()

	// Unique namespace for test isolation
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "elementalmachine-test",
		},
	}

	// CAPI Cluster & belonging Machine objects (Normally created by the Core CAPI provider)
	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
		},
	}
	machine := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: &testBootstrapSecretName,
			},
			ClusterName: "test",
		},
	}
	// ElementalMachine owned by the CAPI Machine (ownership set after creation in BeforeAll())
	elementalMachine := v1beta1.ElementalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
			Labels:    map[string]string{"cluster.x-k8s.io/cluster-name": cluster.Name},
		},
	}

	// installedHost is "installed" and ready to be bootstrapped.
	// We expect this host to be selected for association.
	installedHost := v1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-installed",
			Namespace: namespace.Name,
			Labels:    map[string]string{v1beta1.LabelElementalHostInstalled: "true"},
		},
	}

	// bootstrappedHost is "installed" and already "bootstrapped"
	// Should never be associated (because already bootstrapped)
	bootstrappedHost := v1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bootstrapped",
			Namespace: namespace.Name,
			Labels: map[string]string{
				v1beta1.LabelElementalHostInstalled:    "true",
				v1beta1.LabelElementalHostBootstrapped: "true",
				v1beta1.LabelElementalHostMachineName:  "any-machine",
			},
		},
	}

	// undergoingResetHost is an "installed" host, but scheduled for reset.
	// For this reason it should never be associated to any machine.
	undergoingResetHost := v1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-undergoing-reset",
			Namespace: namespace.Name,
			Labels: map[string]string{
				v1beta1.LabelElementalHostInstalled:  "true",
				v1beta1.LabelElementalHostNeedsReset: "true",
			},
		},
	}

	BeforeAll(func() {
		// Create namespace
		Expect(k8sClient.Create(ctx, &namespace)).Should(Succeed())

		// Create CAPI Cluster and mark it as Infrastructure Ready
		Expect(k8sClient.Create(ctx, &cluster)).Should(Succeed())
		clusterStatusPatch := cluster
		clusterStatusPatch.Status = clusterv1.ClusterStatus{
			InfrastructureReady: true,
		}
		patchObject(ctx, k8sClient, &cluster, &clusterStatusPatch)

		// Create CAPI Machine and owned ElementalMachine to be associated
		Expect(k8sClient.Create(ctx, &machine)).Should(Succeed())
		elementalMachine.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Machine",
			Name:       machine.Name,
			UID:        machine.UID,
		}}
		Expect(k8sClient.Create(ctx, &elementalMachine)).Should(Succeed())
		// Create a bunch of hosts
		Expect(k8sClient.Create(ctx, &installedHost)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &bootstrappedHost)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &undergoingResetHost)).Should(Succeed())
	})
	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, &namespace)).Should(Succeed())
	})
	It("should associate to any installed elemental host", func() {
		wantHostRef := corev1.ObjectReference{
			APIVersion: v1beta1.GroupVersion.Identifier(),
			Kind:       "ElementalHost",
			Namespace:  installedHost.Namespace,
			Name:       installedHost.Name,
			UID:        installedHost.UID,
		}
		Eventually(func() *corev1.ObjectReference {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			return elementalMachine.Spec.HostRef
		}).WithTimeout(time.Minute).ShouldNot(BeNil(), "HostRef must be updated")
		Expect(*elementalMachine.Spec.HostRef).Should(Equal(wantHostRef))

		Expect(elementalMachine.Status.Ready).To(BeFalse(), "ElementalMachine should not be ready as the host is not bootstrapped yet")

		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      installedHost.Name,
			Namespace: installedHost.Namespace,
		}, &installedHost)).Should(Succeed())
		Expect(installedHost.Labels[v1beta1.LabelElementalHostMachineName]).Should(Equal(elementalMachine.Name), "machine-name label must be set")
	})
	It("should not mark machine as ready until cluster's controlplane is initialized ", func() {
		// Mark the host as bootstrapped
		installedHost.Labels[v1beta1.LabelElementalHostBootstrapped] = "true"
		Expect(k8sClient.Update(ctx, &installedHost)).Should(Succeed())

		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			return conditions.IsTrue(&elementalMachine, v1beta1.HostReady)
		}).WithTimeout(time.Minute).Should(BeTrue(), "HostReady condition should be true")

		Expect(elementalMachine.Status.Ready).Should(BeFalse(), "ElementalMachine should not be ready as the cluster controlplane is not initialized yet")
	})
	It("should mark machine as ready after setting ProviderID", func() {
		wantProviderID := fmt.Sprintf("elemental://%s/%s", installedHost.Namespace, installedHost.Name)
		wantCluster := types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}
		wantNodeName := installedHost.Name

		// Expect call on remote tracker to set downstream node's ProviderID
		remoteTrackerMock.AddCall(wantCluster, utils.RemoteTrackerMockCall{NodeName: wantNodeName, ProviderID: wantProviderID})

		// Mark the Cluster's ControlPlaneInitialized
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&cluster), &cluster)).Should(Succeed())
		conditions.Set(&cluster, &clusterv1.Condition{
			Type:   clusterv1.ControlPlaneInitializedCondition,
			Status: v1.ConditionTrue,
		})
		Expect(k8sClient.Status().Update(ctx, &cluster))
		// Add a dummy ControlPlaneRef
		cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{}
		Expect(k8sClient.Update(ctx, &cluster)).Should(Succeed())

		// Ensure ElementalMachine ProviderID matches
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			if elementalMachine.Spec.ProviderID == nil {
				return ""
			}
			return *elementalMachine.Spec.ProviderID
		}).WithTimeout(time.Minute).Should(Equal(wantProviderID), "ElementalMachine's ProviderID should match")

		// Ensure ElementalMachine is ready
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			return elementalMachine.Status.Ready
		}).WithTimeout(time.Minute).Should(BeTrue(), "ElementalMachine should be ready")
	})
	It("should trigger host reset upon deletion", func() {
		// Delete the ElementalMachine
		Expect(k8sClient.Delete(ctx, &elementalMachine)).Should(Succeed())
		// The associated host is expected to start resetting
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      installedHost.Name,
				Namespace: installedHost.Namespace},
				&installedHost)).Should(Succeed())
			if value, found := installedHost.Labels[v1beta1.LabelElementalHostNeedsReset]; found && value == "true" {
				return true
			}
			return false
		}).WithTimeout(time.Minute).Should(BeTrue(), "Host needs.reset label should be set")
	})
})

var _ = Describe("ElementalMachine controller association with selector", Label("controller", "elemental-machine"), Ordered, func() {
	ctx := context.Background()

	// Unique namespace for test isolation
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "elementalmachine-test-with-selector",
		},
	}

	// CAPI Cluster & belonging Machine objects (Normally created by the Core CAPI provider)
	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
		},
	}
	machine := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: &testBootstrapSecretName,
			},
			ClusterName: "test",
		},
	}
	// ElementalMachine owned by the CAPI Machine (ownership set after creation in BeforeAll())
	elementalMachine := v1beta1.ElementalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
			Labels:    map[string]string{"cluster.x-k8s.io/cluster-name": cluster.Name},
		},
		Spec: v1beta1.ElementalMachineSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"foo": "bar"},
			},
		},
	}

	// installedHost is "installed" and ready to be bootstrapped.
	// It has no labels so it should not be selected for association.
	installedHost := v1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-installed",
			Namespace: namespace.Name,
			Labels: map[string]string{
				v1beta1.LabelElementalHostInstalled: "true",
			},
		},
	}

	// installedHostWithLabel is "installed" and ready to be bootstrapped.
	// It has a matching label for association
	installedHostWithLabel := v1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-installed-with-label",
			Namespace: namespace.Name,
			Labels: map[string]string{
				v1beta1.LabelElementalHostInstalled: "true",
				"foo":                               "bar",
			},
		},
	}

	BeforeAll(func() {
		// Create namespace
		Expect(k8sClient.Create(ctx, &namespace)).Should(Succeed())

		// Create CAPI Cluster and mark it as Infrastructure Ready
		Expect(k8sClient.Create(ctx, &cluster)).Should(Succeed())
		clusterStatusPatch := cluster
		clusterStatusPatch.Status = clusterv1.ClusterStatus{
			InfrastructureReady: true,
		}
		patchObject(ctx, k8sClient, &cluster, &clusterStatusPatch)

		// Create CAPI Machine and owned ElementalMachine to be associated
		Expect(k8sClient.Create(ctx, &machine)).Should(Succeed())
		elementalMachine.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Machine",
			Name:       machine.Name,
			UID:        machine.UID,
		}}
		Expect(k8sClient.Create(ctx, &elementalMachine)).Should(Succeed())
		// Create a bunch of hosts
		Expect(k8sClient.Create(ctx, &installedHost)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &installedHostWithLabel)).Should(Succeed())
	})
	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, &namespace)).Should(Succeed())
	})
	It("should use label selector when specified", func() {
		wantHostRef := corev1.ObjectReference{
			APIVersion: v1beta1.GroupVersion.Identifier(),
			Kind:       "ElementalHost",
			Namespace:  installedHostWithLabel.Namespace,
			Name:       installedHostWithLabel.Name,
			UID:        installedHostWithLabel.UID,
		}
		Eventually(func() *corev1.ObjectReference {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			return elementalMachine.Spec.HostRef
		}).WithTimeout(time.Minute).ShouldNot(BeNil(), "HostRef must be updated")
		Expect(*elementalMachine.Spec.HostRef).Should(Equal(wantHostRef))
	})
})

// This test sets up an "already provisioned environment" and breaks the association between ElementalMachine and ElementalHost.
var _ = Describe("ElementalMachine controller association break", Label("controller", "elemental-machine"), Ordered, func() {
	ctx := context.Background()

	// Unique namespace for test isolation
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "elementalmachine-test-association-break",
		},
	}

	// CAPI Cluster (Normally created by the Core CAPI provider)
	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
		},
	}

	// alreadyAssociatedMachine is a CAPI Machine already associated to an ElementalMachine.
	alreadyAssociatedMachine := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-already-associated",
			Namespace: namespace.Name,
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: &testBootstrapSecretName,
			},
			ClusterName: "test",
		},
	}

	// alreadyAssociatedElementalMachine is an ElementalMachine already associated to an ElementalHost
	// and owned by the above CAPI Machine (ownership set in BeforeAll())
	providerID := fmt.Sprintf("elemental://%s/%s", namespace.Name, "test-already-associated")
	alreadyAssociatedElementalMachine := v1beta1.ElementalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-already-associated",
			Namespace: namespace.Name,
			Labels:    map[string]string{"cluster.x-k8s.io/cluster-name": cluster.Name},
		},
		Spec: v1beta1.ElementalMachineSpec{
			ProviderID: &providerID,
			HostRef: &corev1.ObjectReference{
				Kind:       "ElementalHost",
				APIVersion: v1beta1.GroupVersion.Identifier(),
				Name:       "test-already-associated",
				Namespace:  namespace.Name,
			},
		},
	}

	// alreadyAssociatedHost is an ElementalHost associated to the above ElementalMachine
	alreadyAssociatedHost := v1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-already-associated",
			Namespace: namespace.Name,
			Labels: map[string]string{
				v1beta1.LabelElementalHostInstalled:    "true",
				v1beta1.LabelElementalHostBootstrapped: "true",
				v1beta1.LabelElementalHostMachineName:  alreadyAssociatedElementalMachine.Name,
			},
		},
		Spec: v1beta1.ElementalHostSpec{
			MachineRef: &corev1.ObjectReference{
				Kind:       alreadyAssociatedElementalMachine.Kind,
				APIVersion: alreadyAssociatedElementalMachine.APIVersion,
				Name:       alreadyAssociatedElementalMachine.Name,
				Namespace:  alreadyAssociatedElementalMachine.Namespace,
			},
		},
	}

	BeforeAll(func() {
		// Create namespace
		Expect(k8sClient.Create(ctx, &namespace)).Should(Succeed())

		// Create CAPI Cluster and mark it as Infrastructure Ready
		Expect(k8sClient.Create(ctx, &cluster)).Should(Succeed())
		clusterStatusPatch := cluster
		clusterStatusPatch.Status = clusterv1.ClusterStatus{
			InfrastructureReady: true,
		}
		patchObject(ctx, k8sClient, &cluster, &clusterStatusPatch)

		// Create already associated CAPI Machine <--> ElementalMachine <--> ElementalHost
		Expect(k8sClient.Create(ctx, &alreadyAssociatedMachine)).Should(Succeed())
		alreadyAssociatedElementalMachine.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Machine",
			Name:       alreadyAssociatedMachine.Name,
			UID:        alreadyAssociatedMachine.UID,
		}}
		Expect(k8sClient.Create(ctx, &alreadyAssociatedElementalMachine)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &alreadyAssociatedHost)).Should(Succeed())
	})
	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, &namespace)).Should(Succeed())
	})
	It("should remove association if host is deleted", func() {
		// Trigger host deletion
		Expect(k8sClient.Delete(ctx, &alreadyAssociatedHost)).Should(Succeed())

		Eventually(func() *string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      alreadyAssociatedElementalMachine.Name,
				Namespace: alreadyAssociatedElementalMachine.Namespace},
				&alreadyAssociatedElementalMachine)).Should(Succeed())
			return alreadyAssociatedElementalMachine.Spec.ProviderID
		}).WithTimeout(time.Minute).Should(BeNil(), "ProviderID should have been removed")
		Eventually(func() *corev1.ObjectReference {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      alreadyAssociatedElementalMachine.Name,
				Namespace: alreadyAssociatedElementalMachine.Namespace},
				&alreadyAssociatedElementalMachine)).Should(Succeed())
			return alreadyAssociatedElementalMachine.Spec.HostRef
		}).WithTimeout(time.Minute).Should(BeNil(), "HostRef should have been removed")
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      alreadyAssociatedElementalMachine.Name,
				Namespace: alreadyAssociatedElementalMachine.Namespace},
				&alreadyAssociatedElementalMachine)).Should(Succeed())
			return alreadyAssociatedElementalMachine.Status.Ready
		}).WithTimeout(time.Minute).Should(BeFalse(), "ElementalMachine should not be Ready")
	})
})

var _ = Describe("ElementalMachine controller conditions", Label("controller", "elemental-machine", "conditions"), Ordered, func() {
	ctx := context.Background()
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "elementalmachine-test-conditions",
		},
	}
	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
		},
	}
	machine := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: "test",
			InfrastructureRef: corev1.ObjectReference{
				Kind:       "ElementalMachine",
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				Name:       "test",
				Namespace:  namespace.Name,
			},
		},
	}
	elementalMachine := v1beta1.ElementalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
			Labels:    map[string]string{"cluster.x-k8s.io/cluster-name": cluster.Name},
		},
	}

	// installedHost is "installed" and ready to be bootstrapped.
	// We expect this host to be selected for association.
	installedHost := v1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-installed",
			Namespace: namespace.Name,
			Labels:    map[string]string{v1beta1.LabelElementalHostInstalled: "true"},
		},
	}

	// newHost is going to be associated to the machine after deletion of installedHost
	newHost := v1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-new",
			Namespace: namespace.Name,
			Labels:    map[string]string{v1beta1.LabelElementalHostInstalled: "true"},
		},
	}

	BeforeAll(func() {
		// Create namespace
		Expect(k8sClient.Create(ctx, &namespace)).Should(Succeed())
		// Create ElementalMachine
		Expect(k8sClient.Create(ctx, &elementalMachine)).Should(Succeed())
	})
	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, &namespace)).Should(Succeed())
	})
	It("should have MissingMachineOwnerReason", func() {
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady)
			if condition == nil {
				return ""
			}
			return condition.Reason
		}).WithTimeout(time.Minute).Should(Equal(infrastructurev1beta1.MissingMachineOwnerReason))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady).Status).Should(Equal(corev1.ConditionFalse), "AssociationReady condition must be false")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionFalse), "Conditions summary should be false")
	})
	It("should have MissingAssociatedClusterReason", func() {
		// Create CAPI Machine and link ElementalMachine through ownership
		Expect(k8sClient.Create(ctx, &machine)).Should(Succeed())
		elementalMachine.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Machine",
			Name:       machine.Name,
			UID:        machine.UID,
		}}
		Expect(k8sClient.Update(ctx, &elementalMachine)).Should(Succeed())
		// Next failure reason should be MissingAssociatedClusterReason
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady)
			if condition == nil {
				return ""
			}
			return condition.Reason
		}).WithTimeout(time.Minute).Should(Equal(infrastructurev1beta1.MissingAssociatedClusterReason))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady).Status).Should(Equal(corev1.ConditionFalse), "AssociationReady condition must be false")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionFalse), "Conditions summary should be false")
	})
	It("should have MissingClusterInfrastructureReadyReason", func() {
		// Create the associated cluster
		Expect(k8sClient.Create(ctx, &cluster)).Should(Succeed())
		// Next failure reason should be MissingClusterInfrastructureReadyReason
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady)
			if condition == nil {
				return ""
			}
			return condition.Reason
		}).WithTimeout(time.Minute).Should(Equal(infrastructurev1beta1.MissingClusterInfrastructureReadyReason))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady).Status).Should(Equal(corev1.ConditionFalse), "AssociationReady condition must be false")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionFalse), "Conditions summary should be false")
	})
	It("should have MissingBootstrapSecretReason", func() {
		// Patch the cluster as InfrastructureReady
		cluster.Status = clusterv1.ClusterStatus{
			InfrastructureReady: true,
		}
		Expect(k8sClient.Status().Update(ctx, &cluster))
		// Next failure reason should be MissingClusterInfrastructureReadyReason
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady)
			if condition == nil {
				return ""
			}
			return condition.Reason
		}).WithTimeout(time.Minute).Should(Equal(infrastructurev1beta1.MissingBootstrapSecretReason))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady).Status).Should(Equal(corev1.ConditionFalse), "AssociationReady condition must be false")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionFalse), "Conditions summary should be false")
	})
	It("should have MissingAvailableHostsReason", func() {
		// Patch the machine to reference any bootstrap secret
		machine.Spec.Bootstrap = clusterv1.Bootstrap{
			DataSecretName: &testBootstrapSecretName,
		}
		Expect(k8sClient.Update(ctx, &machine)).Should(Succeed())
		// Next failure reason should be MissingAvailableHostsReason
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady)
			if condition == nil {
				return ""
			}
			return condition.Reason
		}).WithTimeout(time.Minute).Should(Equal(infrastructurev1beta1.MissingAvailableHostsReason))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady).Status).Should(Equal(corev1.ConditionFalse), "AssociationReady condition must be false")
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady).Severity).Should(Equal(clusterv1.ConditionSeverityWarning), "Severity should be warning")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionFalse), "Conditions summary should be false")
	})
	It("should have AssociationReady true", func() {
		// Create one installed host
		Expect(k8sClient.Create(ctx, &installedHost)).Should(Succeed())
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionTrue), "AssociationReady condition must be true")
		// After association, we should be awaiting for the Host to be bootstrapped
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.HostReady)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionFalse), "HostReady condition must be false")
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.HostReady).Reason).Should(Equal(infrastructurev1beta1.HostWaitingForBootstrapReason))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.HostReady).Severity).Should(Equal(infrastructurev1beta1.HostWaitingForBootstrapReasonSeverity))
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionFalse), "Conditions summary should be false")
	})
	It("should have HostReady true", func() {
		// Patch the host as bootstrapped
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      installedHost.Name,
			Namespace: installedHost.Namespace},
			&installedHost)).Should(Succeed())
		installedHost.Labels[infrastructurev1beta1.LabelElementalHostBootstrapped] = "true"
		Expect(k8sClient.Update(ctx, &installedHost)).Should(Succeed())
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.HostReady)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionTrue), "HostReady condition must be true")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionFalse), "Conditions summary should be false")
	})
	It("should have HostReady false if host was uninstalled", func() {
		// Remove host installed label. This should not be a possible scenario in normal circumstances.
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      installedHost.Name,
			Namespace: installedHost.Namespace},
			&installedHost)).Should(Succeed())
		delete(installedHost.Labels, infrastructurev1beta1.LabelElementalHostInstalled)
		Expect(k8sClient.Update(ctx, &installedHost)).Should(Succeed())
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.HostReady)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionFalse), "HostReady condition must be false")
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.HostReady).Reason).Should(Equal(infrastructurev1beta1.HostWaitingForInstallReason))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.HostReady).Severity).Should(Equal(clusterv1.ConditionSeverityError))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady)).ShouldNot(BeNil(), "AssociationReady condition must be present")
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady).Status).Should(Equal(corev1.ConditionTrue), "AssociationReady condition must be true")
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, clusterv1.ReadyCondition)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionFalse), "Conditions summary should be false")
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      elementalMachine.Name,
			Namespace: elementalMachine.Namespace},
			&elementalMachine)).Should(Succeed())
		Expect(elementalMachine.Status.Ready).Should(BeFalse())
	})
	It("should have AssociationReady false if host was deleted", func() {
		// Delete the host
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      installedHost.Name,
			Namespace: installedHost.Namespace},
			&installedHost)).Should(Succeed())
		installedHost.Labels[infrastructurev1beta1.LabelElementalHostReset] = "true" // Mark it as already reset
		Expect(k8sClient.Update(ctx, &installedHost)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, &installedHost)).Should(Succeed())
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionFalse), "AssociationReady condition must be false")
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady).Reason).Should(Equal(infrastructurev1beta1.AssociatedHostNotFoundReason))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady).Severity).Should(Equal(infrastructurev1beta1.AssociatedHostNotFoundReasonSeverity))
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionFalse), "Conditions summary should be false")
		Expect(elementalMachine.Status.Ready).Should(BeFalse())
	})
	It("should have AssociationReady true if new host was provisioned", func() {
		// Add a new host
		Expect(k8sClient.Create(ctx, &newHost)).Should(Succeed())
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.AssociationReady)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionTrue), "AssociationReady condition must be true")
		// After association, we should be awaiting for the Host to be bootstrapped
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.HostReady)
			if condition == nil {
				return ""
			}
			return condition.Reason
		}).WithTimeout(time.Minute).Should(Equal(infrastructurev1beta1.HostWaitingForBootstrapReason), "HostReady condition must have HostWaitingForBootstrapReason reason")
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.HostReady).Status).Should(Equal(corev1.ConditionFalse))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.HostReady).Severity).Should(Equal(infrastructurev1beta1.HostWaitingForBootstrapReasonSeverity))
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionFalse), "Conditions summary should be false")
		Expect(elementalMachine.Status.Ready).Should(BeFalse())
	})
	It("should have HostReady true if new host was bootstrapped", func() {
		// Patch the host as bootstrapped
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      newHost.Name,
			Namespace: newHost.Namespace},
			&newHost)).Should(Succeed())
		newHost.Labels[infrastructurev1beta1.LabelElementalHostBootstrapped] = "true"
		Expect(k8sClient.Update(ctx, &newHost)).Should(Succeed())
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, infrastructurev1beta1.HostReady)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionTrue), "HostReady condition must be true")

	})
	It("ProviderIDReady should have WaitingForControlPlaneReason", func() {
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.ProviderIDReady)).ShouldNot(BeNil())
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.ProviderIDReady).Status).Should(Equal(corev1.ConditionFalse))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.ProviderIDReady).Reason).Should(Equal(infrastructurev1beta1.WaitingForControlPlaneReason))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.ProviderIDReady).Severity).Should(Equal(infrastructurev1beta1.WaitingForControlPlaneReasonSeverity))
	})
	It("ProviderIDReady should have NodeNotFoundReason", func() {
		wantCluster := types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}
		// Expect call on remote tracker to set downstream node's ProviderID
		remoteTrackerMock.AddCall(wantCluster, utils.RemoteTrackerMockCall{NodeName: "not-found", ProviderID: ""})
		// Mark the Cluster's ControlPlaneInitialized
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&cluster), &cluster)).Should(Succeed())
		conditions.Set(&cluster, &clusterv1.Condition{
			Type:   clusterv1.ControlPlaneInitializedCondition,
			Status: v1.ConditionTrue,
		})
		Expect(k8sClient.Status().Update(ctx, &cluster))
		// Add a dummy ControlPlaneRef
		cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{}
		Expect(k8sClient.Update(ctx, &cluster)).Should(Succeed())

		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			return conditions.Get(&elementalMachine, infrastructurev1beta1.ProviderIDReady).Reason
		}).WithTimeout(time.Minute).Should(Equal(infrastructurev1beta1.NodeNotFoundReason), "ProviderIDReady should have NodeNotFound reason")
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.ProviderIDReady).Status).Should(Equal(corev1.ConditionFalse))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.ProviderIDReady).Reason).Should(Equal(infrastructurev1beta1.NodeNotFoundReason))
		Expect(conditions.Get(&elementalMachine, infrastructurev1beta1.ProviderIDReady).Severity).Should(Equal(clusterv1.ConditionSeverityError))
	})
	It("ProviderIDReady should be true", func() {
		wantProviderID := fmt.Sprintf("elemental://%s/%s", newHost.Namespace, newHost.Name)
		wantCluster := types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}
		wantNodeName := newHost.Name

		// Expect call on remote tracker to set downstream node's ProviderID
		remoteTrackerMock.AddCall(wantCluster, utils.RemoteTrackerMockCall{NodeName: wantNodeName, ProviderID: wantProviderID})

		Eventually(func() v1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			return conditions.Get(&elementalMachine, infrastructurev1beta1.ProviderIDReady).Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionTrue), "ProviderIDReady should have true status")
		// Test Conditions summary
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			condition := conditions.Get(&elementalMachine, clusterv1.ReadyCondition)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionTrue), "Conditions summary should be true")
		// Ensure ElementalMachine is ready
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			return elementalMachine.Status.Ready
		}).WithTimeout(time.Minute).Should(BeTrue(), "ElementalMachine should be ready")
	})
})
