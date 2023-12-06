package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	var registrationToken string
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
		registrationToken = updatedRegistration.Spec.Config.Elemental.Registration.Token
		Expect(k8sClient.Create(ctx, &bootstrapSecret)).Should(Succeed())
		eClient = client.NewClient("v0.0.0-test")
		conf := config.Config{
			Registration: registration.Spec.Config.Elemental.Registration,
			Agent:        registration.Spec.Config.Elemental.Agent,
		}
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
		Expect(eClient.CreateHost(request, registrationToken)).Should(Succeed())
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
	})
	It("should receive needs reset flag", func() {
		// Add the bootstrap secret reference
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
