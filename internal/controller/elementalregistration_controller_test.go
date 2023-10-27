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
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
)

var _ = Describe("ElementalRegistration controller", Label("controller", "elemental-registration"), Ordered, func() {
	ctx := context.Background()
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "registration-test",
		},
	}
	registration := v1beta1.ElementalRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
		},
	}

	BeforeAll(func() {
		Expect(k8sClient.Create(ctx, &namespace)).Should(Succeed())
	})
	BeforeEach(func() {
		registrationToCreate := registration
		Expect(k8sClient.Create(ctx, &registrationToCreate)).Should(Succeed())
	})
	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &registration)).Should(Succeed())
	})
	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, &namespace)).Should(Succeed())
	})
	It("should set URI if empty", func() {
		// Initial Registration has empty URI field.
		// This is the normal state.
		updatedRegistration := &v1beta1.ElementalRegistration{}
		expectedURI := fmt.Sprintf("%s%s%s/namespaces/%s/registrations/%s", serverURL, api.Prefix, api.PrefixV1, registration.Namespace, registration.Name)
		// Wait for the controller to set the expected URI.
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registration.Name,
				Namespace: registration.Namespace},
				updatedRegistration)).Should(Succeed())
			return updatedRegistration.Spec.Config.Elemental.Registration.URI
		}).WithTimeout(time.Minute).Should(Equal(expectedURI))
	})
	It("should not override URI if already set", func() {
		// Create a new registration with the URI already set.
		// This can be done by the end user if they wish to expose the Elemental API
		// on several load balancers and customize where the underlying agents will connect to.
		registrationWithURI := registration
		registrationWithURI.Name = registration.Name + "-with-uri"
		registrationWithURI.Spec.Config.Elemental.Registration.URI = "just for testing"
		Expect(k8sClient.Create(ctx, &registrationWithURI)).Should(Succeed())

		// Let's trigger a patch just to ensure the controller will be triggered.
		patchHelper, err := patch.NewHelper(&registrationWithURI, k8sClient)
		Expect(err).ToNot(HaveOccurred())
		registrationWithURI.Spec.Config.Elemental.Registration.CACert = "just to trigger the controller"
		Expect(patchHelper.Patch(ctx, &registrationWithURI)).Should(Succeed())
		updatedRegistration := &v1beta1.ElementalRegistration{}
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registrationWithURI.Name,
				Namespace: registrationWithURI.Namespace},
				updatedRegistration)).Should(Succeed())
			return updatedRegistration.Spec.Config.Elemental.Registration.CACert
		}).WithTimeout(time.Minute).Should(Equal(registrationWithURI.Spec.Config.Elemental.Registration.CACert))

		// Verify the initial URI did not change
		Expect(updatedRegistration.Spec.Config.Elemental.Registration.URI).
			To(Equal(registrationWithURI.Spec.Config.Elemental.Registration.URI))
	})
})

var _ = Describe("Elemental API Registration controller", Label("api", "elemental-registration"), Ordered, func() {
	ctx := context.Background()
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "registration-test-client",
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
						CACert: `-----BEGIN CERTIFICATE-----
MIIBvDCCAWOgAwIBAgIBADAKBggqhkjOPQQDAjBGMRwwGgYDVQQKExNkeW5hbWlj
bGlzdGVuZXItb3JnMSYwJAYDVQQDDB1keW5hbWljbGlzdGVuZXItY2FAMTY5NzEy
NjgwNTAeFw0yMzEwMTIxNjA2NDVaFw0zMzEwMDkxNjA2NDVaMEYxHDAaBgNVBAoT
E2R5bmFtaWNsaXN0ZW5lci1vcmcxJjAkBgNVBAMMHWR5bmFtaWNsaXN0ZW5lci1j
YUAxNjk3MTI2ODA1MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE9KvZXqQ7+hN/
4T0LVsFogfENa7UeSI3egvhg54qA6kI4ROQj0sObkbuBbepgGEcaOw8eJW0+M4o3
+SnprKYPkqNCMEAwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8wHQYD
VR0OBBYEFD8W3gE6pK1EjnBM/kPaQF3Uqkc1MAoGCCqGSM49BAMCA0cAMEQCIDxz
wcHkvD3kEU33TR9VnkHUwgC9jDyDa62sef84S5MUAiAJfWf5G5PqtN+AE4XJgg2K
+ETPIs22tcmXyYOG0WY7KQ==
-----END CERTIFICATE-----`,
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

	var fs vfs.FS
	var err error
	var fsCleanup func()
	BeforeAll(func() {
		Expect(k8sClient.Create(ctx, &namespace)).Should(Succeed())
	})
	BeforeEach(func() {
		registrationToCreate := registration
		Expect(k8sClient.Create(ctx, &registrationToCreate)).Should(Succeed())
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(fsCleanup)
	})
	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &registration)).Should(Succeed())
	})
	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, &namespace)).Should(Succeed())
	})
	It("should return expected Registration Response", func() {
		client := client.NewClient()
		conf := config.Config{
			Registration: registration.Spec.Config.Elemental.Registration,
			Agent:        registration.Spec.Config.Elemental.Agent,
		}
		expected := api.RegistrationResponse{
			Config: v1beta1.Config{
				Elemental: v1beta1.Elemental{
					Registration: v1beta1.Registration{
						URI:    "http://localhost:9191/elemental/v1/namespaces/registration-test-client/registrations/test-client",
						CACert: "-----BEGIN CERTIFICATE-----\nMIIBvDCCAWOgAwIBAgIBADAKBggqhkjOPQQDAjBGMRwwGgYDVQQKExNkeW5hbWlj\nbGlzdGVuZXItb3JnMSYwJAYDVQQDDB1keW5hbWljbGlzdGVuZXItY2FAMTY5NzEy\nNjgwNTAeFw0yMzEwMTIxNjA2NDVaFw0zMzEwMDkxNjA2NDVaMEYxHDAaBgNVBAoT\nE2R5bmFtaWNsaXN0ZW5lci1vcmcxJjAkBgNVBAMMHWR5bmFtaWNsaXN0ZW5lci1j\nYUAxNjk3MTI2ODA1MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE9KvZXqQ7+hN/\n4T0LVsFogfENa7UeSI3egvhg54qA6kI4ROQj0sObkbuBbepgGEcaOw8eJW0+M4o3\n+SnprKYPkqNCMEAwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8wHQYD\nVR0OBBYEFD8W3gE6pK1EjnBM/kPaQF3Uqkc1MAoGCCqGSM49BAMCA0cAMEQCIDxz\nwcHkvD3kEU33TR9VnkHUwgC9jDyDa62sef84S5MUAiAJfWf5G5PqtN+AE4XJgg2K\n+ETPIs22tcmXyYOG0WY7KQ==\n-----END CERTIFICATE-----",
					},
					Agent: v1beta1.Agent{
						WorkDir:           "/var/lib/elemental/agent",
						Debug:             true,
						Installer:         "unmanaged",
						Reconciliation:    10000000000,
						InsecureAllowHTTP: true,
					},
				},
			},
		}
		Expect(client.Init(fs, conf)).Should(Succeed())
		// Test API client by fetching the Registration
		registrationResponse, err := client.GetRegistration()
		Expect(err).ToNot(HaveOccurred())
		Expect(registrationResponse).To(Equal(expected))
	})
	It("should return error if namespace or registration not found", func() {
		client := client.NewClient()
		wrongNamespaceURI := fmt.Sprintf("%s%s%s/namespaces/%s/registrations/%s", serverURL, api.Prefix, api.PrefixV1, "does-not-exist", registration.Name)
		conf := config.Config{
			Registration: v1beta1.Registration{URI: wrongNamespaceURI},
			Agent:        registration.Spec.Config.Elemental.Agent,
		}
		Expect(client.Init(fs, conf)).Should(Succeed())
		// Expect err on wrong namespace
		_, err := client.GetRegistration()
		Expect(err).To(HaveOccurred())

		wrongRegistrationURI := fmt.Sprintf("%s%s%s/namespaces/%s/registrations/%s", serverURL, api.Prefix, api.PrefixV1, namespace.Name, "does-not-exist")
		conf.Registration.URI = wrongRegistrationURI
		Expect(client.Init(fs, conf)).Should(Succeed())
		// Expect err on wrong registration name
		_, err = client.GetRegistration()
		Expect(err).To(HaveOccurred())
	})
})
