package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

var (
	testBootstrapSecretName = "test-secret"
)

var _ = Describe("ElementalMachine controller", Label("controller", "elemental-machine"), Ordered, func() {
	ctx := context.Background()
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "elementalmachine-test",
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
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: &testBootstrapSecretName,
			},
			ClusterName: "test",
		},
	}
	elementalMachine := v1beta1.ElementalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
			Labels:    map[string]string{"cluster.x-k8s.io/cluster-name": cluster.Name},
		},
	}
	host := v1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
		},
	}
	// Add one installed host
	installedHost := host
	installedHost.ObjectMeta.Name = "test-installed"
	installedHost.Labels = map[string]string{v1beta1.LabelElementalHostInstalled: "true"}

	// Add one installed host, but already associated to an elemental machine
	additionalMachine := machine
	additionalMachine.ObjectMeta.Name = "test-additional"

	alreadyAssociatedMachine := elementalMachine
	alreadyAssociatedMachine.ObjectMeta.Name = "test-already-associated"
	providerID := fmt.Sprintf("elemental://%s/%s", host.Namespace, "test-already-associated")
	alreadyAssociatedMachine.Spec.ProviderID = &providerID
	alreadyAssociatedMachine.Spec.HostRef = &corev1.ObjectReference{
		Kind:       host.Kind,
		APIVersion: host.APIVersion,
		Name:       "test-already-associated",
		Namespace:  host.Namespace,
	}

	alreadyAssociatedHost := host
	alreadyAssociatedHost.ObjectMeta.Name = "test-already-associated"
	alreadyAssociatedHost.Labels = map[string]string{v1beta1.LabelElementalHostInstalled: "true"}
	alreadyAssociatedHost.Labels[v1beta1.LabelElementalHostBootstrapped] = "true"
	alreadyAssociatedHost.Labels[v1beta1.LabelElementalHostMachineName] = alreadyAssociatedMachine.Name
	alreadyAssociatedHost.Spec.MachineRef = &corev1.ObjectReference{
		Kind:       alreadyAssociatedMachine.Kind,
		APIVersion: alreadyAssociatedMachine.APIVersion,
		Name:       alreadyAssociatedMachine.Name,
		Namespace:  alreadyAssociatedMachine.Namespace,
	}

	// Add one installed and bootstrapped host
	bootstrappedHost := host
	bootstrappedHost.ObjectMeta.Name = "test-bootstrapped"
	bootstrappedHost.Labels = map[string]string{v1beta1.LabelElementalHostInstalled: "true"}
	bootstrappedHost.Labels[v1beta1.LabelElementalHostBootstrapped] = "true"
	bootstrappedHost.Labels[v1beta1.LabelElementalHostMachineName] = "any-machine"

	// Add one installed and bootstrapped host undergoing reset
	undergoingResetHost := host
	undergoingResetHost.ObjectMeta.Name = "test-undergoing-reset"
	undergoingResetHost.Labels = map[string]string{v1beta1.LabelElementalHostInstalled: "true"}
	undergoingResetHost.Labels[v1beta1.LabelElementalHostBootstrapped] = "true"
	undergoingResetHost.Labels[v1beta1.LabelElementalHostNeedsReset] = "true"

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
		Expect(k8sClient.Create(ctx, &alreadyAssociatedHost)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &additionalMachine)).Should(Succeed())
		alreadyAssociatedMachine.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Machine",
			Name:       additionalMachine.Name,
			UID:        additionalMachine.UID,
		}}
		Expect(k8sClient.Create(ctx, &alreadyAssociatedMachine)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &bootstrappedHost)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &undergoingResetHost)).Should(Succeed())
	})
	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, &namespace)).Should(Succeed())
	})
	It("should associate to any installed elemental host", func() {
		updatedMachine := &v1beta1.ElementalMachine{}
		wantProviderID := fmt.Sprintf("elemental://%s/%s", installedHost.Namespace, installedHost.Name)
		wantHostRef := corev1.ObjectReference{
			APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			Kind:       "ElementalHost",
			Namespace:  installedHost.Namespace,
			Name:       installedHost.Name,
			UID:        installedHost.UID,
		}
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				updatedMachine)).Should(Succeed())
			if updatedMachine.Spec.ProviderID == nil {
				return ""
			}
			return *updatedMachine.Spec.ProviderID
		}).WithTimeout(time.Minute).Should(Equal(wantProviderID), "ProviderID must be updated")
		Eventually(func() corev1.ObjectReference {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				updatedMachine)).Should(Succeed())
			return *updatedMachine.Spec.HostRef
		}).WithTimeout(time.Minute).Should(Equal(wantHostRef), "HostRef must be updated")

		updatedHost := &v1beta1.ElementalHost{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      installedHost.Name,
			Namespace: installedHost.Namespace,
		}, updatedHost)).Should(Succeed())
		Expect(updatedHost.Labels[v1beta1.LabelElementalHostMachineName]).Should(Equal(elementalMachine.Name), "machine-name label must be set")

		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				updatedMachine)).Should(Succeed())
			return updatedMachine.Status.Ready
		}).WithTimeout(time.Minute).Should(BeFalse(), "ElementalMachine should not be ready as the host is not bootstrapped yet")

		// Now mark the host as bootstrapped
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      installedHost.Name,
			Namespace: installedHost.Namespace,
		}, updatedHost)).Should(Succeed())
		Eventually(func() error {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      installedHost.Name,
				Namespace: installedHost.Namespace,
			}, updatedHost)).Should(Succeed())
			updatedHost.Labels[v1beta1.LabelElementalHostBootstrapped] = "true"
			return k8sClient.Update(ctx, updatedHost)
		}).WithTimeout(time.Minute).Should(BeNil())
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				updatedMachine)).Should(Succeed())
			return updatedMachine.Status.Ready
		}).WithTimeout(time.Minute).Should(BeTrue(), "ElementalMachine should be ready")
	})
	It("should mark already associated machine as ready", func() {
		updatedMachine := &v1beta1.ElementalMachine{}
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      alreadyAssociatedMachine.Name,
				Namespace: alreadyAssociatedMachine.Namespace},
				updatedMachine)).Should(Succeed())
			return updatedMachine.Status.Ready
		}).WithTimeout(time.Minute).Should(BeTrue())
	})
	It("should remove association if host is deleted", func() {
		// Mark the host as reset to remove finalized and enable deletion
		updatedHost := &v1beta1.ElementalHost{}
		Eventually(func() error {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      alreadyAssociatedHost.Name,
				Namespace: alreadyAssociatedHost.Namespace,
			}, updatedHost)).Should(Succeed())
			updatedHost.Labels[v1beta1.LabelElementalHostReset] = "true"
			return k8sClient.Update(ctx, updatedHost)
		}).WithTimeout(time.Minute).Should(BeNil())
		Expect(k8sClient.Delete(ctx, updatedHost)).Should(Succeed())

		updatedMachine := &v1beta1.ElementalMachine{}
		Eventually(func() *string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      alreadyAssociatedMachine.Name,
				Namespace: alreadyAssociatedMachine.Namespace},
				updatedMachine)).Should(Succeed())
			return updatedMachine.Spec.ProviderID
		}).WithTimeout(time.Minute).Should(BeNil())
		Eventually(func() *corev1.ObjectReference {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      alreadyAssociatedMachine.Name,
				Namespace: alreadyAssociatedMachine.Namespace},
				updatedMachine)).Should(Succeed())
			return updatedMachine.Spec.HostRef
		}).WithTimeout(time.Minute).Should(BeNil())
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      alreadyAssociatedMachine.Name,
				Namespace: alreadyAssociatedMachine.Namespace},
				updatedMachine)).Should(Succeed())
			return updatedMachine.Status.Ready
		}).WithTimeout(time.Minute).Should(BeFalse())
		// Remove this machine as no longer needed for further tests
		Expect(k8sClient.Delete(ctx, &alreadyAssociatedMachine)).Should(Succeed())
	})
	It("should trigger host reset upon deletion", func() {
		Expect(k8sClient.Delete(ctx, &elementalMachine)).Should(Succeed())
		updatedHost := &v1beta1.ElementalHost{}
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      installedHost.Name,
				Namespace: installedHost.Namespace},
				updatedHost)).Should(Succeed())
			if value, found := updatedHost.Labels[v1beta1.LabelElementalHostNeedsReset]; found && value == "true" {
				return true
			}
			return false
		}).WithTimeout(time.Minute).Should(BeTrue())
	})
	It("should use label selector when specified", func() {
		// Add new CAPI Machine
		newMachine := clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-new",
				Namespace: namespace.Name,
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: &testBootstrapSecretName,
				},
				ClusterName: "test",
			},
		}
		Expect(k8sClient.Create(ctx, &newMachine)).Should(Succeed())
		// Add owned ElementalMachine with selector
		elementalMachineWithSelector := v1beta1.ElementalMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-with-selector",
				Namespace: namespace.Name,
				Labels:    map[string]string{"cluster.x-k8s.io/cluster-name": cluster.Name},
			},
			Spec: v1beta1.ElementalMachineSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
			},
		}
		elementalMachineWithSelector.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Machine",
			Name:       newMachine.Name,
			UID:        newMachine.UID,
		}}
		Expect(k8sClient.Create(ctx, &elementalMachineWithSelector)).Should(Succeed())
		// Add installed host with selector
		hostWithSelector := host
		hostWithSelector.ObjectMeta.Name = "test-with-selector"
		hostWithSelector.Labels = map[string]string{v1beta1.LabelElementalHostInstalled: "true"}
		hostWithSelector.Labels["foo"] = "bar"
		Expect(k8sClient.Create(ctx, &hostWithSelector)).Should(Succeed())

		updatedMachine := &v1beta1.ElementalMachine{}
		wantProviderID := fmt.Sprintf("elemental://%s/%s", hostWithSelector.Namespace, hostWithSelector.Name)
		wantHostRef := corev1.ObjectReference{
			APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			Kind:       "ElementalHost",
			Namespace:  hostWithSelector.Namespace,
			Name:       hostWithSelector.Name,
			UID:        hostWithSelector.UID,
		}
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachineWithSelector.Name,
				Namespace: elementalMachineWithSelector.Namespace},
				updatedMachine)).Should(Succeed())
			if updatedMachine.Spec.ProviderID == nil {
				return ""
			}
			return *updatedMachine.Spec.ProviderID
		}).WithTimeout(time.Minute).Should(Equal(wantProviderID), "ProviderID must be updated")
		Eventually(func() corev1.ObjectReference {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachineWithSelector.Name,
				Namespace: elementalMachineWithSelector.Namespace},
				updatedMachine)).Should(Succeed())
			return *updatedMachine.Spec.HostRef
		}).WithTimeout(time.Minute).Should(Equal(wantHostRef), "HostRef must be updated")
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
	host := v1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
		},
	}
	// Add one installed host
	installedHost := host
	installedHost.ObjectMeta.Name = "test-installed"
	installedHost.Labels = map[string]string{v1beta1.LabelElementalHostInstalled: "true"}
	// New host is used after installedHost deletion
	newHost := host
	installedHost.ObjectMeta.Name = "test-new"
	newHost.Labels = map[string]string{infrastructurev1beta1.LabelElementalHostInstalled: "true"}
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
		elementalMachinePatch := elementalMachine
		elementalMachinePatch.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Machine",
			Name:       machine.Name,
			UID:        machine.UID,
		}}
		patchObject(ctx, k8sClient, &elementalMachine, &elementalMachinePatch)
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
		clusterStatusPatch := cluster
		clusterStatusPatch.Status = clusterv1.ClusterStatus{
			InfrastructureReady: true,
		}
		patchObject(ctx, k8sClient, &cluster, &clusterStatusPatch)
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
		machinePatch := machine
		machinePatch.Spec.Bootstrap = clusterv1.Bootstrap{
			DataSecretName: &testBootstrapSecretName,
		}
		patchObject(ctx, k8sClient, &machine, &machinePatch)
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
		Expect(conditions.Get(&elementalMachine, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionTrue), "Conditions summary should be true")
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				&elementalMachine)).Should(Succeed())
			return elementalMachine.Status.Ready
		}).WithTimeout(time.Minute).Should(BeTrue(), "ElementalMachine status should be true")
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
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      elementalMachine.Name,
			Namespace: elementalMachine.Namespace},
			&elementalMachine)).Should(Succeed())
		Expect(elementalMachine.Status.Ready).Should(BeTrue())
	})
})
