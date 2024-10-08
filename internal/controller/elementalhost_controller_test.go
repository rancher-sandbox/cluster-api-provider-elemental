package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
)

var _ = Describe("ElementalHost controller", Label("controller", "elemental-host"), Ordered, func() {
	ctx := context.Background()
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "elementalhost-test",
		},
	}
	host := v1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
		},
	}
	BeforeAll(func() {
		Expect(k8sClient.Create(ctx, &namespace)).Should(Succeed())
	})
	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, &namespace)).Should(Succeed())
	})
	It("should not remove finalizer until reset done", func() {
		hostToBeReset := host
		hostToBeReset.ObjectMeta.Name = "test-to-be-reset"
		Expect(k8sClient.Create(ctx, &hostToBeReset)).Should(Succeed())
		// Finalizer should be applied
		updatedHost := &v1beta1.ElementalHost{}
		Eventually(func() []string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      hostToBeReset.Name,
				Namespace: hostToBeReset.Namespace},
				updatedHost)).Should(Succeed())
			return updatedHost.GetFinalizers()
		}).WithTimeout(time.Minute).Should(ContainElement(v1beta1.FinalizerElementalMachine), "ElementalHost should have finalizer")
		// Delete this host
		Expect(k8sClient.Delete(ctx, &hostToBeReset)).Should(Succeed())
		Eventually(func() *metav1.Time {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      hostToBeReset.Name,
				Namespace: hostToBeReset.Namespace},
				updatedHost)).Should(Succeed())
			return updatedHost.GetDeletionTimestamp()
		}).WithTimeout(time.Minute).ShouldNot(BeNil(), "ElementalHost should have deletion timestamp")
		Eventually(func() []string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      hostToBeReset.Name,
				Namespace: hostToBeReset.Namespace},
				updatedHost)).Should(Succeed())
			return updatedHost.GetFinalizers()
		}).WithTimeout(time.Minute).Should(ContainElement(v1beta1.FinalizerElementalMachine), "ElementalHost should still have finalizer after deletion")
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      hostToBeReset.Name,
				Namespace: hostToBeReset.Namespace},
				updatedHost)).Should(Succeed())
			return updatedHost.Labels[v1beta1.LabelElementalHostNeedsReset]
		}).WithTimeout(time.Minute).Should(Equal("true"), "ElementalHost should have needs-reset label")
		resetCondition := conditions.Get(updatedHost, v1beta1.ResetReady)
		Expect(resetCondition).ShouldNot(BeNil())
		Expect(resetCondition.Status).Should(Equal(corev1.ConditionFalse))
		Expect(resetCondition.Severity).Should(Equal(v1beta1.WaitingForResetReasonSeverity))
		Expect(resetCondition.Reason).Should(Equal(v1beta1.WaitingForResetReason))
		Expect(resetCondition.Message).Should(Equal("Waiting for remote host to reset"))
		// Patch with reset done label
		updatedHost.Labels[v1beta1.LabelElementalHostReset] = "true"
		Expect(k8sClient.Update(ctx, updatedHost)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      hostToBeReset.Name,
				Namespace: hostToBeReset.Namespace},
				updatedHost)
			return apierrors.IsNotFound(err)
		}).WithTimeout(time.Minute).Should(BeTrue(), "ElementalHost should be deleted")
	})
	It("should set conditions summary", func() {
		// Create an "already installed" host
		hostWithConditions := host
		hostWithConditions.ObjectMeta.Name = "test-with-conditions"
		hostWithConditions.Labels = map[string]string{v1beta1.LabelElementalHostInstalled: "true"}
		Expect(k8sClient.Create(ctx, &hostWithConditions)).Should(Succeed())
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      hostWithConditions.Name,
				Namespace: hostWithConditions.Namespace},
				&hostWithConditions)).Should(Succeed())
			condition := conditions.Get(&hostWithConditions, v1beta1.InstallationReady)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionTrue), "InstallationReady condition should be true")
		Expect(conditions.Get(&hostWithConditions, v1beta1.RegistrationReady)).ShouldNot(BeNil(), "RegistrationReady condition should be present")
		Expect(conditions.Get(&hostWithConditions, v1beta1.RegistrationReady).Status).Should(Equal(corev1.ConditionTrue), "RegistrationReady condition should be true if host is installed")
		Expect(conditions.Get(&hostWithConditions, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&hostWithConditions, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionTrue), "Conditions summary should be true")
		// Set one unready condition and expect summary to be false
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      hostWithConditions.Name,
			Namespace: hostWithConditions.Namespace},
			&hostWithConditions)).Should(Succeed())
		hostWithConditionsPatch := hostWithConditions
		conditions.Set(&hostWithConditionsPatch, &clusterv1.Condition{
			Type:     v1beta1.BootstrapReady,
			Status:   corev1.ConditionFalse,
			Severity: clusterv1.ConditionSeverityError,
			Reason:   "test reason",
			Message:  "just for testing",
		})
		patchObject(ctx, k8sClient, &hostWithConditions, &hostWithConditionsPatch)
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      hostWithConditions.Name,
				Namespace: hostWithConditions.Namespace},
				&hostWithConditions)).Should(Succeed())
			condition := conditions.Get(&hostWithConditions, clusterv1.ReadyCondition)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionTrue), "Conditions summary should be false")
		// Set bootstrapped label
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      hostWithConditions.Name,
			Namespace: hostWithConditions.Namespace},
			&hostWithConditions)).Should(Succeed())
		hostWithConditionsPatch = hostWithConditions
		hostWithConditionsPatch.Labels = map[string]string{v1beta1.LabelElementalHostBootstrapped: "true"}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      hostWithConditions.Name,
			Namespace: hostWithConditions.Namespace},
			&hostWithConditions)).Should(Succeed())
		patchObject(ctx, k8sClient, &hostWithConditions, &hostWithConditionsPatch)
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      hostWithConditions.Name,
				Namespace: hostWithConditions.Namespace},
				&hostWithConditions)).Should(Succeed())
			condition := conditions.Get(&hostWithConditions, v1beta1.BootstrapReady)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionTrue), "BootstrapReady condition should be true")
		Expect(conditions.Get(&hostWithConditions, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&hostWithConditions, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionTrue), "Conditions summary should be true")
	})
	It("should reconcile OS version from associated Elemental Machine", func() {
		// Create a new to-be-associated ElementalHost first
		associatedHost := host
		associatedHost.ObjectMeta.Name = "test-os-reconcile"
		associatedHost.Labels = map[string]string{v1beta1.LabelElementalHostInstalled: "true"}
		Expect(k8sClient.Create(ctx, &associatedHost)).Should(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Namespace: associatedHost.Namespace,
			Name:      associatedHost.Name,
		}, &associatedHost)).Should(Succeed())
		// Create and associate the ElementalMachine
		elementalMachine := v1beta1.ElementalMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-os-reconcile",
				Namespace: namespace.Name,
			},
			Spec: v1beta1.ElementalMachineSpec{
				HostRef: &corev1.ObjectReference{
					Kind:       associatedHost.Kind,
					Namespace:  associatedHost.Namespace,
					Name:       associatedHost.Name,
					APIVersion: associatedHost.APIVersion,
					UID:        associatedHost.UID,
				},
				OSVersionManagement: map[string]runtime.RawExtension{
					"test": {Raw: []byte(`"1"`)},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &elementalMachine)).Should(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Namespace: elementalMachine.Namespace,
			Name:      elementalMachine.Name,
		}, &elementalMachine)).Should(Succeed())
		// Double link the ElementalHost to ElementalMachine
		associatedHostPatch := associatedHost
		associatedHostPatch.Spec.MachineRef = &corev1.ObjectReference{
			Kind:       elementalMachine.Kind,
			Namespace:  elementalMachine.Namespace,
			Name:       elementalMachine.Name,
			APIVersion: elementalMachine.APIVersion,
			UID:        elementalMachine.UID,
		}
		patchObject(ctx, k8sClient, &associatedHost, &associatedHostPatch)
		// Verify OSVersionManagement has been propagated from the ElementalMachine
		Eventually(func() map[string]runtime.RawExtension {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      associatedHost.Name,
				Namespace: associatedHost.Namespace},
				&associatedHost)).Should(Succeed())
			return associatedHost.Spec.OSVersionManagement
		}).WithTimeout(time.Minute).Should(Equal(elementalMachine.Spec.OSVersionManagement),
			"OSVersionManagement should have been propagated from ElementalMachine")
		osVersionReadyCondition := conditions.Get(&associatedHost, v1beta1.OSVersionReady)
		Expect(osVersionReadyCondition.Status).Should(Equal(v1.ConditionFalse))
		Expect(osVersionReadyCondition.Reason).Should(Equal(v1beta1.WaitingOSReconcileReason))
		Expect(osVersionReadyCondition.Severity).Should(Equal(v1beta1.WaitingOSReconcileReasonSeverity))
		Expect(osVersionReadyCondition.Message).Should(Equal(fmt.Sprintf("ElementalMachine %s OSVersionManagement mutated", elementalMachine.Name)))
		// Mark the host as bootstrapped now. This will be needed to test a different OSVersionReady false reason
		associatedHostPatch = associatedHost
		associatedHostPatch.Labels = map[string]string{v1beta1.LabelElementalHostBootstrapped: "true"}
		patchObject(ctx, k8sClient, &associatedHost, &associatedHostPatch)
		// Mutate the OSVersion on the ElementalMachine
		elementalMachinePatch := elementalMachine
		elementalMachinePatch.Spec.OSVersionManagement = map[string]runtime.RawExtension{
			"foo": {Raw: []byte(`"2"`)},
		}
		patchObject(ctx, k8sClient, &elementalMachine, &elementalMachinePatch)
		// Verify OSVersionManagement has been propagated again
		Eventually(func() map[string]runtime.RawExtension {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      associatedHost.Name,
				Namespace: associatedHost.Namespace},
				&associatedHost)).Should(Succeed())
			return associatedHost.Spec.OSVersionManagement
		}).WithTimeout(time.Minute).Should(Equal(elementalMachinePatch.Spec.OSVersionManagement),
			"Updated OSVersionManagement should have been propagated from ElementalMachine")
		osVersionReadyCondition = conditions.Get(&associatedHost, v1beta1.OSVersionReady)
		Expect(osVersionReadyCondition.Status).Should(Equal(v1.ConditionFalse))
		Expect(osVersionReadyCondition.Reason).Should(Equal(v1beta1.InPlaceUpdateNotPendingReason))
		Expect(osVersionReadyCondition.Severity).Should(Equal(v1beta1.InPlaceUpdateNotPendingReasonSeverity))
		Expect(osVersionReadyCondition.Message).Should(Equal(fmt.Sprintf("ElementalMachine %s OSVersionManagement mutated, but no in-place-upgrade is pending. Mutation will be ignored.", elementalMachine.Name)))
	})
})

var _ = Describe("Elemental API Host controller", Label("api", "elemental-host"), Ordered, func() {
	ctx := context.Background()
	trueVar := true
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "host-test-client",
		},
	}
	registration := v1beta1.ElementalRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-client",
			Namespace: namespace.Name,
		},
		Spec: v1beta1.ElementalRegistrationSpec{
			Config: v1beta1.Config{
				Elemental: v1beta1.Elemental{
					Registration: v1beta1.Registration{
						URI: fmt.Sprintf("%s%s%s/namespaces/%s/registrations/%s", serverURL, api.Prefix, api.PrefixV1, namespace.Name, "test-client"),
					},
					Agent: v1beta1.Agent{
						WorkDir:           "/var/lib/elemental/agent",
						Debug:             true,
						InsecureAllowHTTP: true,
					},
				},
			},
		},
	}
	bootstrapFormat := "cloud-config"
	bootstrapConfig := `#cloud-config
write_files:
-   path: '/tmp/test-file'
	owner: 'root:root'
	permissions: '0640'
	content: 'JUST FOR TESTING'
runcmd:
- 'echo testing'`
	bootstrapSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bootstrap",
			Namespace: namespace.Name,
		},
		Type: "cluster.x-k8s.io/secret",
		StringData: map[string]string{
			"format": bootstrapFormat,
			"value":  bootstrapConfig,
		},
	}
	request := api.HostCreateRequest{
		Name:        "test-new",
		Annotations: map[string]string{"foo": "bar"},
		Labels:      map[string]string{"bar": "foo"},
	}
	var fs vfs.FS
	var err error
	var fsCleanup func()
	var eClient client.Client
	BeforeAll(func() {
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(fsCleanup)
		Expect(k8sClient.Create(ctx, &namespace)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &registration)).Should(Succeed())
		updatedRegistration := &v1beta1.ElementalRegistration{}
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registration.Name,
				Namespace: registration.Namespace},
				updatedRegistration)).Should(Succeed())
			return len(updatedRegistration.Spec.Config.Elemental.Registration.Token) != 0
		}).WithTimeout(time.Minute).Should(BeTrue(), "missing registration token")
		registrationToken := updatedRegistration.Spec.Config.Elemental.Registration.Token
		Expect(k8sClient.Create(ctx, &bootstrapSecret)).Should(Succeed())
		eClient = client.NewClient("v0.0.0-test")
		conf := config.Config{
			Registration: registration.Spec.Config.Elemental.Registration,
			Agent:        registration.Spec.Config.Elemental.Agent,
		}
		conf.Registration.Token = registrationToken
		idManager := identity.NewManager(fs, registration.Spec.Config.Elemental.Agent.WorkDir)
		id, err := idManager.LoadSigningKeyOrCreateNew()
		Expect(err).ToNot(HaveOccurred())
		pubKey, err := id.MarshalPublic()
		Expect(err).ToNot(HaveOccurred())
		request.PubKey = string(pubKey)
		Expect(eClient.Init(fs, id, conf)).Should(Succeed())
	})
	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, &namespace)).Should(Succeed())
	})
	It("should create new host", func() {
		// Create the new host
		Expect(eClient.CreateHost(request)).Should(Succeed())
		// Issue an empty patch to get a host response
		response, err := eClient.PatchHost(api.HostPatchRequest{}, request.Name)
		Expect(err).ToNot(HaveOccurred())
		wantResponse := api.HostResponse{
			Name:        request.Name,
			Annotations: request.Annotations,
			Labels:      request.Labels,
		}
		Expect(*response).To(Equal(wantResponse))
	})
	It("should patch host with installed label", func() {
		// Patch the host as Installed
		response, err := eClient.PatchHost(api.HostPatchRequest{Installed: &trueVar}, request.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.Installed).To(BeTrue())
		// Verify label is applied
		updatedHost := &v1beta1.ElementalHost{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      request.Name,
			Namespace: namespace.Name},
			updatedHost)).Should(Succeed())
		value, found := updatedHost.Labels[v1beta1.LabelElementalHostInstalled]
		Expect(found).Should(BeTrue(), "Installed label must be present")
		Expect(value).Should(Equal("true"), "Installed label must have 'true' value")
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      request.Name,
				Namespace: namespace.Name},
				updatedHost)).Should(Succeed())
			registeredCondition := conditions.Get(updatedHost, v1beta1.RegistrationReady)
			installedCondition := conditions.Get(updatedHost, v1beta1.InstallationReady)
			return registeredCondition != nil && registeredCondition.Status == corev1.ConditionTrue && installedCondition != nil && installedCondition.Status == corev1.ConditionTrue
		}).WithTimeout(time.Minute).Should(BeTrue(), "conditions must be set")
	})
	It("should fail to get bootstrap if bootstrap secret not ready yet", func() {
		_, err := eClient.GetBootstrap(request.Name)
		Expect(err).To(HaveOccurred())
	})
	It("should fail to bootstrap host if bootstrap secret not ready yet", func() {
		// Patch the host as bootstrapped
		_, err := eClient.PatchHost(api.HostPatchRequest{Bootstrapped: &trueVar}, request.Name)
		Expect(err).To(HaveOccurred())
	})
	It("should receive bootstrap ready flag if bootstrap secret available", func() {
		// Add the bootstrap secret reference
		host := &v1beta1.ElementalHost{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      request.Name,
			Namespace: namespace.Name},
			host)).Should(Succeed())
		host.Spec.BootstrapSecret = &corev1.ObjectReference{
			Kind:       bootstrapSecret.Kind,
			APIVersion: bootstrapSecret.APIVersion,
			Name:       bootstrapSecret.Name,
			Namespace:  bootstrapSecret.Namespace,
			UID:        bootstrapSecret.UID,
		}
		Expect(k8sClient.Update(ctx, host)).Should(Succeed())
		// Issue an empty patch to get a host response
		response, err := eClient.PatchHost(api.HostPatchRequest{}, request.Name)
		Expect(err).ToNot(HaveOccurred())
		// Verify bootstrap ready flag is true
		Expect(response.BootstrapReady).Should(BeTrue(), "Boostrap ready must be true")
	})
	It("should get bootstrap if bootstrap ready", func() {
		bootstrapResponse, err := eClient.GetBootstrap(request.Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(bootstrapResponse.Format).Should(Equal(bootstrapFormat))
		Expect(bootstrapResponse.Config).Should(Equal(bootstrapConfig))
	})
	It("should patch host with bootstrapped label", func() {
		// Patch the host as bootstrapped
		response, err := eClient.PatchHost(api.HostPatchRequest{Bootstrapped: &trueVar}, request.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.Bootstrapped).To(BeTrue())
		// Verify label is applied
		updatedHost := &v1beta1.ElementalHost{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      request.Name,
			Namespace: namespace.Name},
			updatedHost)).Should(Succeed())
		value, found := updatedHost.Labels[v1beta1.LabelElementalHostInstalled]
		Expect(found).Should(BeTrue(), "Bootstrapped label must be present")
		Expect(value).Should(Equal("true"), "Bootstrapped label must have 'true' value")
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      request.Name,
				Namespace: namespace.Name},
				updatedHost)).Should(Succeed())
			bootstrappedCondition := conditions.Get(updatedHost, v1beta1.BootstrapReady)
			return bootstrappedCondition != nil && bootstrappedCondition.Status == corev1.ConditionTrue
		}).WithTimeout(time.Minute).Should(BeTrue(), "condition must be set")
	})
	It("should receive needs reset flag", func() {
		host := &v1beta1.ElementalHost{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      request.Name,
			Namespace: namespace.Name},
			host)).Should(Succeed())
		host.Labels[v1beta1.LabelElementalHostNeedsReset] = "true"
		Expect(k8sClient.Update(ctx, host)).Should(Succeed())
		// Issue an empty patch to get a host response
		response, err := eClient.PatchHost(api.HostPatchRequest{}, request.Name)
		Expect(err).ToNot(HaveOccurred())
		// Verify needs reset flag is true
		Expect(response.NeedsReset).Should(BeTrue(), "Needs reset must be true")
	})
	It("should trigger ElementalHost deletion on delete", func() {
		// Delete the host from the client
		Expect(eClient.DeleteHost(request.Name)).Should(Succeed())
		// Verify the host has a deletion timestamp
		host := &v1beta1.ElementalHost{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      request.Name,
			Namespace: namespace.Name},
			host)).Should(Succeed())
		Expect(host.GetDeletionTimestamp()).ShouldNot(BeNil())
		Expect(host.GetDeletionTimestamp().IsZero()).ShouldNot(BeTrue())
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      request.Name,
				Namespace: namespace.Name},
				host)).Should(Succeed())
			value, found := host.Labels[v1beta1.LabelElementalHostNeedsReset]
			return found && value == "true"
		}).WithTimeout(time.Minute).Should(BeTrue(), "needs reset label must be set")
	})
	It("should remove ElementalHost finalizer on reset complete", func() {
		// Patch the host as reset done
		_, err := eClient.PatchHost(api.HostPatchRequest{Reset: &trueVar}, request.Name)
		Expect(err).ToNot(HaveOccurred())
		// Verify it has been deleted
		host := &v1beta1.ElementalHost{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      request.Name,
				Namespace: namespace.Name},
				host)
			return apierrors.IsNotFound(err)
		}).WithTimeout(time.Minute).Should(BeTrue(), "ElementalHost should be deleted")
	})
})
