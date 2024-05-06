package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/config"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity"
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
		deletionPropagation := metav1.DeletePropagationForeground
		Expect(k8sClient.Delete(ctx, &namespace, &ctrlclient.DeleteOptions{
			GracePeriodSeconds: ptr.To(int64(0)),
			PropagationPolicy:  &deletionPropagation,
		})).Should(Succeed())
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
	It("should set Ready condition", func() {
		updatedRegistration := &v1beta1.ElementalRegistration{}
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registration.Name,
				Namespace: registration.Namespace},
				updatedRegistration)).Should(Succeed())
			return conditions.IsTrue(updatedRegistration, clusterv1.ReadyCondition)
		}).WithTimeout(time.Minute).Should(BeTrue(), "Registration should have Ready condition")
	})
	It("should not override URI if already set", func() {
		// Create a new registration with the URI already set.
		// This can be done by the end user if they wish to expose the Elemental API
		// on several load balancers and customize where the underlying agents will connect to.
		registrationWithURI := registration
		registrationWithURI.Name = registration.Name + "-with-uri"
		registrationWithURI.Spec.Config.Elemental.Registration.URI = "just for testing"
		Expect(k8sClient.Create(ctx, &registrationWithURI)).Should(Succeed())
		// Verify the initial URI did not change
		updatedRegistration := &v1beta1.ElementalRegistration{}
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registrationWithURI.Name,
				Namespace: registrationWithURI.Namespace},
				updatedRegistration)).Should(Succeed())
			return updatedRegistration.Spec.Config.Elemental.Registration.URI
		}).WithTimeout(time.Minute).Should(Equal(registrationWithURI.Spec.Config.Elemental.Registration.URI))
	})
	It("should create non-expirable registration token by default", func() {
		// Initial Registration has empty token.
		// This is the normal state.
		updatedRegistration := &v1beta1.ElementalRegistration{}
		// Wait for the controller to create a new token.
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registration.Name,
				Namespace: registration.Namespace},
				updatedRegistration)).Should(Succeed())
			return len(updatedRegistration.Spec.Config.Elemental.Registration.Token) != 0
		}).WithTimeout(time.Minute).Should(BeTrue(), "registration token should be created")

		token := updatedRegistration.Spec.Config.Elemental.Registration.Token
		expectedClaims := &jwt.RegisteredClaims{
			Subject:  registration.Spec.Config.Elemental.Registration.URI,
			Audience: []string{registration.Spec.Config.Elemental.Registration.URI},
		}
		parser := jwt.Parser{}
		parsedToken, _, err := parser.ParseUnverified(token, expectedClaims)
		Expect(err).ToNot(HaveOccurred())
		expirationTime, err := parsedToken.Claims.GetExpirationTime()
		Expect(err).ToNot(HaveOccurred())
		Expect(expirationTime).Should(BeNil(), "registration token should not expire")
	})
	It("should not override already created token", func() {
		registrationWithToken := registration
		registrationWithToken.Name = registration.Name + "-with-token"
		registrationWithToken.Spec.Config.Elemental.Registration.Token = "just a test token"
		Expect(k8sClient.Create(ctx, &registrationWithToken)).Should(Succeed())
		// Verify the initial token did not change
		updatedRegistration := &v1beta1.ElementalRegistration{}
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registrationWithToken.Name,
				Namespace: registrationWithToken.Namespace},
				updatedRegistration)).Should(Succeed())
			return updatedRegistration.Spec.Config.Elemental.Registration.Token
		}).WithTimeout(time.Minute).Should(Equal(registrationWithToken.Spec.Config.Elemental.Registration.Token))
	})
	It("should generate new token after token is deleted", func() {
		// Initial Registration has empty token.
		// This is the normal state.
		updatedRegistration := &v1beta1.ElementalRegistration{}
		// Wait for the controller to create a new token.
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registration.Name,
				Namespace: registration.Namespace},
				updatedRegistration)).Should(Succeed())
			return len(updatedRegistration.Spec.Config.Elemental.Registration.Token) != 0
		}).WithTimeout(time.Minute).Should(BeTrue(), "registration token should be created")
		// Delete the token
		patchHelper, err := patch.NewHelper(updatedRegistration, k8sClient)
		Expect(err).ToNot(HaveOccurred())
		updatedRegistration.Spec.Config.Elemental.Registration.Token = ""
		Expect(patchHelper.Patch(ctx, updatedRegistration)).Should(Succeed())
		// Ensure token is re-created
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registration.Name,
				Namespace: registration.Namespace},
				updatedRegistration)).Should(Succeed())
			return len(updatedRegistration.Spec.Config.Elemental.Registration.Token) != 0
		}).WithTimeout(time.Minute).Should(BeTrue(), "registration token should be created")
	})
	It("should generate an already expired token if duration is negative", func() {
		registrationWithExpiredToken := registration
		registrationWithExpiredToken.Name = registration.Name + "-with-expired-token"
		registrationWithExpiredToken.Spec.Config.Elemental.Registration.TokenDuration = -1
		Expect(k8sClient.Create(ctx, &registrationWithExpiredToken)).Should(Succeed())
		updatedRegistration := &v1beta1.ElementalRegistration{}
		// Wait for the controller to create a new token.
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registrationWithExpiredToken.Name,
				Namespace: registrationWithExpiredToken.Namespace},
				updatedRegistration)).Should(Succeed())
			return len(updatedRegistration.Spec.Config.Elemental.Registration.Token) != 0
		}).WithTimeout(time.Minute).Should(BeTrue(), "registration token should be created")

		token := updatedRegistration.Spec.Config.Elemental.Registration.Token
		expectedClaims := &jwt.RegisteredClaims{
			Subject:  registration.Spec.Config.Elemental.Registration.URI,
			Audience: []string{registration.Spec.Config.Elemental.Registration.URI},
		}
		parser := jwt.Parser{}
		parsedToken, _, err := parser.ParseUnverified(token, expectedClaims)
		Expect(err).ToNot(HaveOccurred())
		expirationTime, err := parsedToken.Claims.GetExpirationTime()
		Expect(err).ToNot(HaveOccurred())
		Expect(expirationTime).ShouldNot(BeNil(), "epiration time should be set")
		Expect(expirationTime.Before(time.Now())).Should(BeTrue(), "registration token should be expired")
	})
	It("should update CACert with default value", func() {
		Eventually(func() string {
			updatedRegistration := &v1beta1.ElementalRegistration{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registration.Name,
				Namespace: registration.Namespace},
				updatedRegistration)).Should(Succeed())
			return updatedRegistration.Spec.Config.Elemental.Registration.CACert
		}).WithTimeout(time.Minute).Should(Equal(testCAValue))
	})
	It("should not override already defined CACert", func() {
		caCert := "an already defined CA cert"
		registrationWithCACert := registration
		registrationWithCACert.Name = registration.Name + "-with-ca-cert"
		registrationWithCACert.Spec.Config.Elemental.Registration.CACert = caCert
		Expect(k8sClient.Create(ctx, &registrationWithCACert)).Should(Succeed())
		// Verify CACert didn't change
		updatedRegistration := &v1beta1.ElementalRegistration{}
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registrationWithCACert.Name,
				Namespace: registrationWithCACert.Namespace},
				updatedRegistration)).Should(Succeed())
			return updatedRegistration.Spec.Config.Elemental.Registration.CACert
		}).WithTimeout(time.Minute).Should(Equal(caCert))
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
	var eClient client.Client
	var id identity.Identity
	var registrationToken string
	BeforeAll(func() {
		Expect(k8sClient.Create(ctx, &namespace)).Should(Succeed())
	})
	BeforeEach(func() {
		registrationToCreate := registration
		Expect(k8sClient.Create(ctx, &registrationToCreate)).Should(Succeed())
		updatedRegistration := &v1beta1.ElementalRegistration{}
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registration.Name,
				Namespace: registration.Namespace},
				updatedRegistration)).Should(Succeed())
			return len(updatedRegistration.Spec.Config.Elemental.Registration.Token) != 0
		}).WithTimeout(time.Minute).Should(BeTrue(), "missing registration token")
		registrationToken = updatedRegistration.Spec.Config.Elemental.Registration.Token
		fs, fsCleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(fsCleanup)
		idManager := identity.NewManager(fs, registration.Spec.Config.Elemental.Agent.WorkDir)
		id, err = idManager.LoadSigningKeyOrCreateNew()
		Expect(err).ToNot(HaveOccurred())
		eClient = client.NewClient("v0.0.0-test")
		Expect(err).ToNot(HaveOccurred())
	})
	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &registration)).Should(Succeed())
	})
	AfterAll(func() {
		deletionPropagation := metav1.DeletePropagationForeground
		Expect(k8sClient.Delete(ctx, &namespace, &ctrlclient.DeleteOptions{
			GracePeriodSeconds: ptr.To(int64(0)),
			PropagationPolicy:  &deletionPropagation,
		})).Should(Succeed())
	})
	It("should return expected Registration Response", func() {
		conf := config.Config{
			Registration: registration.Spec.Config.Elemental.Registration,
			Agent:        registration.Spec.Config.Elemental.Agent,
		}
		conf.Registration.Token = registrationToken
		expected := api.RegistrationResponse{
			Config: v1beta1.Config{
				Elemental: v1beta1.Elemental{
					Registration: v1beta1.Registration{
						URI:    "http://localhost:9191/elemental/v1/namespaces/registration-test-client/registrations/test-client",
						CACert: "-----BEGIN CERTIFICATE-----\nMIIBvDCCAWOgAwIBAgIBADAKBggqhkjOPQQDAjBGMRwwGgYDVQQKExNkeW5hbWlj\nbGlzdGVuZXItb3JnMSYwJAYDVQQDDB1keW5hbWljbGlzdGVuZXItY2FAMTY5NzEy\nNjgwNTAeFw0yMzEwMTIxNjA2NDVaFw0zMzEwMDkxNjA2NDVaMEYxHDAaBgNVBAoT\nE2R5bmFtaWNsaXN0ZW5lci1vcmcxJjAkBgNVBAMMHWR5bmFtaWNsaXN0ZW5lci1j\nYUAxNjk3MTI2ODA1MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE9KvZXqQ7+hN/\n4T0LVsFogfENa7UeSI3egvhg54qA6kI4ROQj0sObkbuBbepgGEcaOw8eJW0+M4o3\n+SnprKYPkqNCMEAwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8wHQYD\nVR0OBBYEFD8W3gE6pK1EjnBM/kPaQF3Uqkc1MAoGCCqGSM49BAMCA0cAMEQCIDxz\nwcHkvD3kEU33TR9VnkHUwgC9jDyDa62sef84S5MUAiAJfWf5G5PqtN+AE4XJgg2K\n+ETPIs22tcmXyYOG0WY7KQ==\n-----END CERTIFICATE-----",
						Token:  registrationToken,
					},
					Agent: v1beta1.Agent{
						WorkDir:           "/var/lib/elemental/agent",
						Debug:             true,
						OSPlugin:          "/usr/lib/elemental/plugins/elemental.so",
						Reconciliation:    10000000000,
						InsecureAllowHTTP: true,
					},
				},
			},
		}
		Expect(eClient.Init(fs, id, conf)).Should(Succeed())
		// Test API client by fetching the Registration
		registrationResponse, err := eClient.GetRegistration()
		Expect(err).ToNot(HaveOccurred())
		Expect(*registrationResponse).To(Equal(expected))
	})
	It("should return error if namespace or registration not found", func() {
		wrongNamespaceURI := fmt.Sprintf("%s%s%s/namespaces/%s/registrations/%s", serverURL, api.Prefix, api.PrefixV1, "does-not-exist", registration.Name)
		conf := config.Config{
			Registration: v1beta1.Registration{URI: wrongNamespaceURI, Token: registrationToken},
			Agent:        registration.Spec.Config.Elemental.Agent,
		}
		Expect(eClient.Init(fs, id, conf)).Should(Succeed())
		// Expect err on wrong namespace
		_, err := eClient.GetRegistration()
		Expect(err).To(HaveOccurred())

		wrongRegistrationURI := fmt.Sprintf("%s%s%s/namespaces/%s/registrations/%s", serverURL, api.Prefix, api.PrefixV1, namespace.Name, "does-not-exist")
		conf.Registration.URI = wrongRegistrationURI
		Expect(eClient.Init(fs, id, conf)).Should(Succeed())
		// Expect err on wrong registration name
		_, err = eClient.GetRegistration()
		Expect(err).To(HaveOccurred())
	})
})
