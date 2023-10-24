package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
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
	It("should remove finalizer if reset not needed", func() {
		hostResetNotNeeded := host
		hostResetNotNeeded.ObjectMeta.Name = "test-reset-not-needed"
		Expect(k8sClient.Create(ctx, &hostResetNotNeeded)).Should(Succeed())
		// Finalizer should be applied
		updatedHost := &v1beta1.ElementalHost{}
		Eventually(func() []string {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      hostResetNotNeeded.Name,
				Namespace: hostResetNotNeeded.Namespace},
				updatedHost)).Should(Succeed())
			return updatedHost.GetFinalizers()
		}).WithTimeout(time.Minute).Should(ContainElement(v1beta1.FinalizerElementalMachine), "ElementalHost should have finalizer")
		// Delete this host without ever setting the needs.reset label
		Expect(k8sClient.Delete(ctx, &hostResetNotNeeded)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      hostResetNotNeeded.Name,
				Namespace: hostResetNotNeeded.Namespace},
				updatedHost)
			return apierrors.IsNotFound(err)
		}).WithTimeout(time.Minute).Should(BeTrue(), "ElementalHost should be deleted")
	})
	It("should not remove finalizer if reset needed, until reset done", func() {
		hostToBeReset := host
		hostToBeReset.ObjectMeta.Name = "test-to-be-reset"
		hostToBeReset.Labels = map[string]string{v1beta1.LabelElementalHostNeedsReset: "true"}
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
