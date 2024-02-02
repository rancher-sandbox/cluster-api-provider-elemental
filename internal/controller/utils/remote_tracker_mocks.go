package utils

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

var _ RemoteTracker = (*RemoteTrackerMock)(nil)

func NewRemoteTrackerMock() *RemoteTrackerMock {
	return &RemoteTrackerMock{
		calls: make(map[types.NamespacedName]RemoteTrackerMockCall),
	}
}

// This is a manually crafted mock.
// The reason of not using gomock is that we need this mock to be "global" and used across
// different tests, which gomock does not support.
type RemoteTrackerMock struct {
	lock  sync.Mutex
	calls map[types.NamespacedName]RemoteTrackerMockCall
}

type RemoteTrackerMockCall struct {
	NodeName   string
	ProviderID string
}

func (r *RemoteTrackerMock) AddCall(cluster types.NamespacedName, call RemoteTrackerMockCall) {
	r.lock.TryLock()
	defer r.lock.Unlock()
	r.calls[cluster] = call
}

func (r *RemoteTrackerMock) SetProviderID(_ context.Context, cluster types.NamespacedName, nodeName string, providerID string) error {
	r.lock.TryLock()
	defer r.lock.Unlock()
	call, found := r.calls[cluster]
	if !found {
		return fmt.Errorf("Cluster %s not found", cluster.String())
	}

	if call.NodeName != nodeName {
		return ErrRemoteNodeNotFound
	}
	if call.ProviderID != providerID {
		return fmt.Errorf("Want ProviderID '%s', but got '%s'", call.ProviderID, providerID)
	}
	return nil
}
