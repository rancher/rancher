package usercontrollers

import (
	"context"
	"fmt"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/peermanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// mockPeerManager is a mock implementation of peermanager.PeerManager
type mockPeerManager struct {
	mock.Mock
	isLeader  bool
	listeners []chan<- peermanager.Peers
}

func (m *mockPeerManager) IsLeader() bool {
	return m.isLeader
}

func (m *mockPeerManager) Leader() {
	m.isLeader = true
}

func (m *mockPeerManager) AddListener(listener chan<- peermanager.Peers) {
	m.listeners = append(m.listeners, listener)
	m.Called(listener)
}

func (m *mockPeerManager) RemoveListener(listener chan<- peermanager.Peers) {
	for i, l := range m.listeners {
		if l == listener {
			m.listeners = append(m.listeners[:i], m.listeners[i+1:]...)
			break
		}
	}
	m.Called(listener)
}

func (m *mockPeerManager) SendPeers(peers peermanager.Peers) {
	for _, listener := range m.listeners {
		listener <- peers
	}
}

func TestNonClusteredStrategy(t *testing.T) {
	strategy := getOwnerStrategy(t.Context(), nil)

	assert.IsType(t, &nonClusteredStrategy{}, strategy)
	assert.Nil(t, strategy.forcedResync(), "forcedResync should return nil for nonClusteredStrategy")
	assert.True(t, strategy.amOwner(&v3.Cluster{}), "nonClusteredStrategy should own all clusters")
}

func TestPeersBasedStrategy(t *testing.T) {
	cluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-1",
			UID:  "cluster-uid-1",
		},
	}

	t.Run("not ready peer should not own", func(t *testing.T) {
		mockPeerManager := new(mockPeerManager)
		mockPeerManager.On("AddListener", mock.Anything).Return()
		mockPeerManager.On("RemoveListener", mock.Anything).Return()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		strategy := getOwnerStrategy(ctx, mockPeerManager)
		assert.IsType(t, &peersBasedStrategy{}, strategy)

		assert.False(t, strategy.amOwner(cluster), "Should not own if not initialized")

		// Simulate initial state where peers are not ready
		mockPeerManager.SendPeers(peermanager.Peers{
			SelfID: "peer1",
			Ready:  false,
			Leader: false,
		})
		// sendPeers is asynchronous, wait for the signal
		err := receiveWithTimeout(100*time.Millisecond, strategy.forcedResync())
		assert.NoError(t, err, "Expected resync channel to be sent")

		assert.False(t, strategy.amOwner(cluster), "Should not own if peer manager is not ready")
	})

	t.Run("single non-leader replica should not own", func(t *testing.T) {
		mockPeerManager := new(mockPeerManager)
		mockPeerManager.On("AddListener", mock.Anything).Return()
		mockPeerManager.On("RemoveListener", mock.Anything).Return()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		strategy := getOwnerStrategy(ctx, mockPeerManager)

		// Simulate a single, non-leader replica
		mockPeerManager.SendPeers(peermanager.Peers{
			SelfID: "peer1",
			Ready:  true,
			Leader: false,
		})
		// sendPeers is asynchronous, wait for the signal
		err := receiveWithTimeout(100*time.Millisecond, strategy.forcedResync())
		assert.NoError(t, err, "Expected resync channel to be sent")

		assert.False(t, strategy.amOwner(cluster), "Single non-leader replica should not own")
	})

	t.Run("ownership distribution with multiple peers", func(t *testing.T) {
		mockPeerManager := new(mockPeerManager)
		mockPeerManager.On("AddListener", mock.Anything).Return()
		mockPeerManager.On("RemoveListener", mock.Anything).Return()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		strategy := getOwnerStrategy(ctx, mockPeerManager)

		// Test various cluster UIDs to check ownership distribution
		testCases := []struct {
			clusterUID    string
			expectedOwner string
		}{
			{clusterUID: "cluster-abc", expectedOwner: "peerC"},
			{clusterUID: "cluster-def", expectedOwner: "peerB"},
			{clusterUID: "cluster-ghi", expectedOwner: "peerC"},
			{clusterUID: "cluster-jkl", expectedOwner: "peerC"},
			{clusterUID: "cluster-mno", expectedOwner: "peerA"},
		}

		// Simulate multiple peers
		peers := []string{"peerA", "peerB", "peerC"}
		// Repeat the test from all the different perspectives
		for x, selfID := range peers {
			mockPeerManager.SendPeers(peermanager.Peers{
				SelfID: selfID,
				IDs:    append(append([]string{}, peers[:x]...), peers[x+1:]...), // omit self
				Ready:  true,
			})
			// sendPeers is asynchronous, wait for the signal
			err := receiveWithTimeout(100*time.Millisecond, strategy.forcedResync())
			assert.NoError(t, err, "Expected resync channel to be sent")

			for _, tc := range testCases {
				t.Run("peer="+selfID+",cluster="+tc.clusterUID, func(t *testing.T) {
					cluster := &v3.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							UID:  types.UID(tc.clusterUID),
							Name: "test-" + tc.clusterUID,
						},
					}

					isOwnerExpected := selfID == tc.expectedOwner
					assert.Equal(t, isOwnerExpected, strategy.amOwner(cluster), "Incorrect cluster ownership")
				})
			}
		}
	})

	t.Run("forcedResync channel sends on peer change", func(t *testing.T) {
		mockPeerManager := new(mockPeerManager)
		mockPeerManager.On("AddListener", mock.Anything).Return()
		mockPeerManager.On("RemoveListener", mock.Anything).Return()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		strategy := getOwnerStrategy(ctx, mockPeerManager)

		// Initial peers
		mockPeerManager.SendPeers(peermanager.Peers{
			SelfID: "peer1",
		})
		err := receiveWithTimeout(100*time.Millisecond, strategy.forcedResync())
		assert.NoError(t, err, "Expected resync channel to be sent")

		// Change peers - expect a resync
		mockPeerManager.SendPeers(peermanager.Peers{
			SelfID: "peer1",
			IDs:    []string{"peer2"},
		})
		err = receiveWithTimeout(100*time.Millisecond, strategy.forcedResync())
		assert.NoError(t, err, "Expected resync channel to be sent")

		// Send same peers again - no resync expected
		mockPeerManager.SendPeers(peermanager.Peers{
			SelfID: "peer1",
			IDs:    []string{"peer2"},
		})
		err = receiveWithTimeout(100*time.Millisecond, strategy.forcedResync())
		assert.Error(t, err, "Unexpected resync")
	})

	t.Run("cleanup on context cancellation", func(t *testing.T) {
		mockPeerManager := new(mockPeerManager)
		addCall := mockPeerManager.On("AddListener", mock.Anything).Return()
		removeCall := mockPeerManager.On("RemoveListener", mock.Anything).Return()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		strategy := getOwnerStrategy(ctx, mockPeerManager)
		mockPeerManager.AssertCalled(t, "AddListener", addCall.Arguments.Get(0))

		// Cancel the context
		cancel()

		// Wait a bit for the goroutine to process the cancellation
		time.Sleep(100 * time.Millisecond)

		// Assert that RemoveListener was called
		mockPeerManager.AssertCalled(t, "RemoveListener", removeCall.Arguments.Get(0))

		var closed bool
		select {
		case _, closed = <-strategy.forcedResync():
		default:
		}
		assert.False(t, closed, "Expected forcedResync channel to be closed")
	})

	t.Run("setPeers handles selfID correctly in IDs list", func(t *testing.T) {
		strategy := &peersBasedStrategy{}
		peers := peermanager.Peers{
			SelfID: "B",
			IDs:    []string{"C", "A"}, // SelfID is not in IDs initially
			Ready:  true,
			Leader: true,
		}
		strategy.setPeers(peers)

		// After setPeers, "B" should be added and the slice sorted.
		expectedIDs := []string{"A", "B", "C"}
		assert.Equal(t, expectedIDs, strategy.peers.IDs, "SelfID should be added and IDs sorted")

		peersWithSelf := peermanager.Peers{
			SelfID: "X",
			IDs:    []string{"X", "Y"}, // SelfID is already in IDs
			Ready:  true,
			Leader: true,
		}
		changed := strategy.setPeers(peersWithSelf)
		expectedIDsWithSelf := []string{"X", "Y"}
		assert.Equal(t, expectedIDsWithSelf, strategy.peers.IDs, "SelfID should not be duplicated if already present")
		assert.True(t, changed, "Expected setPeers to return true")
	})
}

func receiveWithTimeout[T any](timeout time.Duration, ch <-chan T) error {
	select {
	case <-ch:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("Timed out waiting for channel")
	}
}
