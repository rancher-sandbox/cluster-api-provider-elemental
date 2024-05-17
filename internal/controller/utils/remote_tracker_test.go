package utils

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestRegister(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Remote Tracker Suite")
}

var _ = Describe("Remote Tracker", Label("utils", "remote tracker"), Ordered, func() {
	ctx := context.TODO()

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}

	taintedNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-tainted",
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{uninitializedTaint},
		},
	}

	Expect(clusterv1.AddToScheme(scheme.Scheme)).Should(Succeed())
	logger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	fakeClient := fake.NewClientBuilder().WithObjects(cluster, node, taintedNode).Build()
	tracker := remote.NewTestClusterCacheTracker(logger, fakeClient, fakeClient, scheme.Scheme, types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name})

	remoteTracker := NewRemoteTracker(tracker)
	It("should return error if cluster not found", func() {
		Expect(remoteTracker.SetProviderID(ctx,
			types.NamespacedName{Name: "not", Namespace: "found"},
			"foo",
			"bar")).ShouldNot(Succeed())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).Should(Succeed())
		Expect(node.Spec.ProviderID).Should(BeEmpty())
	})
	It("should return ErrRemoteNodeNotFound if node not found", func() {
		Expect(remoteTracker.SetProviderID(ctx,
			client.ObjectKeyFromObject(cluster),
			"foo",
			"bar")).Should(MatchError(ErrRemoteNodeNotFound))
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).Should(Succeed())
		Expect(node.Spec.ProviderID).Should(BeEmpty())
	})
	It("should patch ProviderID on remote node", func() {
		wantProviderID := "elemental://testNamespace/testName"
		Expect(remoteTracker.SetProviderID(ctx,
			client.ObjectKeyFromObject(cluster),
			node.Name,
			wantProviderID)).Should(Succeed())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).Should(Succeed())
		Expect(node.Spec.ProviderID).Should(Equal(wantProviderID))
	})
	It("should patch ProviderID and remove taint on remote tainted node", func() {
		wantProviderID := "elemental://testNamespace/testName"
		Expect(remoteTracker.SetProviderID(ctx,
			client.ObjectKeyFromObject(cluster),
			taintedNode.Name,
			wantProviderID)).Should(Succeed())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(taintedNode), taintedNode)).Should(Succeed())
		Expect(taintedNode.Spec.ProviderID).Should(Equal(wantProviderID))
		Expect(taintedNode.Spec.Taints).Should(BeEmpty())
	})
})
