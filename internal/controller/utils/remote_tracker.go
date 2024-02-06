package utils

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/util/taints"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Errors.
var (
	ErrRemoteNodeNotFound = errors.New("remote node not found")
)

var (
	uninitializedTaint = corev1.Taint{Key: "node.cloudprovider.kubernetes.io/uninitialized", Effect: corev1.TaintEffectNoSchedule}
)

// RemoteTracker wraps a remote.ClusterCacheTracker for easier testing.
type RemoteTracker interface {
	SetProviderID(ctx context.Context, cluster types.NamespacedName, nodeName string, providerID string) error
}

var _ RemoteTracker = (*remoteTracker)(nil)

func NewRemoteTracker(tracker *remote.ClusterCacheTracker) RemoteTracker {
	return &remoteTracker{
		Tracker: tracker,
	}
}

type remoteTracker struct {
	Tracker *remote.ClusterCacheTracker
}

func (r *remoteTracker) SetProviderID(ctx context.Context, cluster types.NamespacedName, nodeName string, providerID string) error {
	remoteClient, err := r.Tracker.GetClient(ctx, cluster)
	if err != nil {
		return fmt.Errorf("getting remote client for cluster '%s/%s': %w", cluster.Namespace, cluster.Name, err)
	}

	node := &corev1.Node{}
	nodeKey := client.ObjectKey{Name: nodeName}
	err = remoteClient.Get(ctx, nodeKey, node)
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("getting node '%s': %w: %w", nodeKey.Name, ErrRemoteNodeNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("getting downstream cluster node '%s': %w", nodeKey.Name, err)
	}

	// Initialize Node patch helper
	patchHelper, err := patch.NewHelper(node, remoteClient)
	if err != nil {
		return fmt.Errorf("initializing node patch helper: %w", err)
	}

	// Set the spec.providerID on the node
	node.Spec.ProviderID = providerID

	// Remove taint if needed
	node, _, err = taints.RemoveTaint(node, &uninitializedTaint)
	if err != nil {
		return fmt.Errorf("removing '%s' taint from node: %w", &uninitializedTaint, err)
	}

	if err := patchHelper.Patch(ctx, node); err != nil {
		return fmt.Errorf("patching downstream cluster node: %w", err)
	}
	return nil
}
