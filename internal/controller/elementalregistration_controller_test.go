package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Registration controller", func() {
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
		Spec: v1beta1.ElementalRegistrationSpec{
			Config: &v1beta1.Config{
				Elemental: v1beta1.Elemental{
					Registration: v1beta1.Registration{
						CACert: "just a CA cert",
					},
					Agent: v1beta1.Agent{
						Debug:             true,
						InsecureAllowHTTP: true,
					},
				},
			},
		},
	}
	BeforeEach(func() {
		Expect(k8sClient.Create(context.Background(), &namespace)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &registration)).Should(Succeed())
	})
	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &registration)).Should(Succeed())
	})
	It("should set URI if empty", func() {
		updatedRegistration := &v1beta1.ElementalRegistration{}
		expectedURI := fmt.Sprintf("%s%s%s/namespaces/%s/registrations/%s", serverURL, api.Prefix, api.PrefixV1, registration.Namespace, registration.Name)
		Eventually(func() string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      registration.Name,
				Namespace: registration.Namespace},
				updatedRegistration)).Should(Succeed())
			return updatedRegistration.Spec.Config.Elemental.Registration.URI
		}, "1m").Should(Equal(expectedURI))
	})
})
