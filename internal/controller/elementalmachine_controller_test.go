package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      elementalMachine.Name,
				Namespace: elementalMachine.Namespace},
				updatedMachine)).Should(Succeed())
			return updatedMachine.Status.Ready
		}).WithTimeout(time.Minute).Should(BeFalse(), "ElementalMachine should not be ready as the host is not bootstrapped yet")

		// Now mark the host as bootstrapped
		updatedHost := &v1beta1.ElementalHost{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      installedHost.Name,
			Namespace: installedHost.Namespace,
		}, updatedHost)).Should(Succeed())
		updatedHost.Labels[v1beta1.LabelElementalHostBootstrapped] = "true"
		Expect(k8sClient.Update(ctx, updatedHost)).Should(Succeed())
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
		hostPatch := alreadyAssociatedHost
		hostPatch.Labels[v1beta1.LabelElementalHostReset] = "true"
		patchObject(ctx, k8sClient, &alreadyAssociatedHost, &hostPatch)
		Expect(k8sClient.Delete(ctx, &alreadyAssociatedHost)).Should(Succeed())

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
