package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

var _ = Describe("ElementalCluster controller", Label("controller", "elemental-cluster"), Ordered, func() {
	ctx := context.Background()
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "elementalcluster-test",
		},
	}
	cluster := v1beta1.ElementalCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace.Name,
		},
	}
	capiCluster := clusterv1.Cluster{
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
	It("should set conditions summary", func() {
		// Create a cluster without CAPI Cluster owner
		Expect(k8sClient.Create(ctx, &cluster)).Should(Succeed())
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace},
				&cluster)).Should(Succeed())
			condition := conditions.Get(&cluster, v1beta1.CAPIClusterReady)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionFalse), "CAPIClusterReady condition should be false")
		Expect(conditions.Get(&cluster, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&cluster, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionFalse), "Conditions summary should be false")
		Expect(cluster.Status.Ready).Should(BeFalse())
		// Create and link the CAPI Cluster
		Expect(k8sClient.Create(ctx, &capiCluster)).Should(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      cluster.Name,
			Namespace: cluster.Namespace},
			&cluster)).Should(Succeed())
		clusterPatch := cluster
		clusterPatch.ObjectMeta.OwnerReferences = append(clusterPatch.ObjectMeta.OwnerReferences, metav1.OwnerReference{
			APIVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Cluster",
			Name:       capiCluster.Name,
			UID:        capiCluster.UID,
		})
		patchObject(ctx, k8sClient, &cluster, &clusterPatch)
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace},
				&cluster)).Should(Succeed())
			condition := conditions.Get(&cluster, v1beta1.CAPIClusterReady)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionTrue), "CAPIClusterReady condition should be true")
		Expect(conditions.Get(&cluster, v1beta1.ControlPlaneEndpointReady)).ShouldNot(BeNil(), "ControlPlaneEndpointReady should be present")
		Expect(conditions.Get(&cluster, v1beta1.ControlPlaneEndpointReady).Status).Should(Equal(corev1.ConditionFalse), "ControlPlaneEndpointReady should be false")
		Expect(conditions.Get(&cluster, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&cluster, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionFalse), "Conditions summary should be false")
		Expect(cluster.Status.Ready).Should(BeFalse())
		// Patch the controlPlaneEndpoint
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      cluster.Name,
			Namespace: cluster.Namespace},
			&cluster)).Should(Succeed())
		clusterPatch = cluster
		clusterPatch.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
			Host: "foo",
			Port: 1234,
		}
		patchObject(ctx, k8sClient, &cluster, &clusterPatch)
		Eventually(func() corev1.ConditionStatus {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace},
				&cluster)).Should(Succeed())
			condition := conditions.Get(&cluster, v1beta1.ControlPlaneEndpointReady)
			if condition == nil {
				return corev1.ConditionUnknown
			}
			return condition.Status
		}).WithTimeout(time.Minute).Should(Equal(corev1.ConditionTrue), "ControlPlaneEndpointReady condition should be true")
		Expect(conditions.Get(&cluster, clusterv1.ReadyCondition)).ShouldNot(BeNil(), "Conditions summary should be present")
		Expect(conditions.Get(&cluster, clusterv1.ReadyCondition).Status).Should(Equal(corev1.ConditionTrue), "Conditions summary should be true")
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace},
				&cluster)).Should(Succeed())
			return cluster.Status.Ready
		}).WithTimeout(time.Minute).Should(BeTrue())
	})
})
