package usercontrollers

//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen -package usercontrollers -source=../../../../pkg/peermanager/peermanager.go -destination=./peermanager_mock_test.go

import (
	"context"
	"fmt"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/peermanager"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func TestNonClusteredStrategy(t *testing.T) {
	strategy := getOwnerStrategy(t.Context(), nil, false)

	assert.IsType(t, &nonClusteredStrategy{}, strategy)
	assert.Nil(t, strategy.forcedResync(), "forcedResync should return nil for nonClusteredStrategy")
	assert.True(t, strategy.isOwner(&v3.Cluster{}), "nonClusteredStrategy should own all clusters")
}

func TestPeersBasedStrategy(t *testing.T) {
	cluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-1",
			UID:  "cluster-uid-1",
		},
	}

	t.Run("not ready peer should not own", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockPeerManager := NewMockPeerManager(ctrl)

		var peersChan chan<- peermanager.Peers
		mockPeerManager.EXPECT().AddListener(gomock.Any()).Do(func(lis chan<- peermanager.Peers) {
			peersChan = lis
		})
		strategy := newPeersBasedStrategy(t.Context(), mockPeerManager, false)
		mockPeerManager.EXPECT().RemoveListener(peersChan).AnyTimes()
		assert.False(t, strategy.isOwner(cluster), "Should not own if not initialized")

		// Simulate initial state where peers are not ready
		peersChan <- peermanager.Peers{
			SelfID: "peer1",
			Ready:  false,
			Leader: false,
		}
		// sendPeers is asynchronous, wait for the signal
		err := receiveWithTimeout(500*time.Millisecond, strategy.forcedResync())
		assert.NoError(t, err, "Expected resync channel to be sent")

		assert.False(t, strategy.isOwner(cluster), "Should not own if peer manager is not ready")
	})

	t.Run("single non-leader replica should not own", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockPeerManager := NewMockPeerManager(ctrl)

		var peersChan chan<- peermanager.Peers
		mockPeerManager.EXPECT().AddListener(gomock.Any()).Do(func(lis chan<- peermanager.Peers) {
			peersChan = lis
		})
		strategy := newPeersBasedStrategy(t.Context(), mockPeerManager, false)
		mockPeerManager.EXPECT().RemoveListener(peersChan).AnyTimes()

		// Simulate a single, non-leader replica
		peersChan <- peermanager.Peers{
			SelfID: "peer1",
			Ready:  true,
			Leader: false,
		}
		// sendPeers is asynchronous, wait for the signal
		err := receiveWithTimeout(500*time.Millisecond, strategy.forcedResync())
		assert.NoError(t, err, "Expected resync channel to be sent")

		assert.False(t, strategy.isOwner(cluster), "Single non-leader replica should not own")
	})

	t.Run("ownership distribution with multiple peers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockPeerManager := NewMockPeerManager(ctrl)

		var peersChan chan<- peermanager.Peers
		mockPeerManager.EXPECT().AddListener(gomock.Any()).Do(func(lis chan<- peermanager.Peers) {
			peersChan = lis
		})
		strategy := newPeersBasedStrategy(t.Context(), mockPeerManager, false)
		mockPeerManager.EXPECT().RemoveListener(peersChan).AnyTimes()

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
			peersChan <- peermanager.Peers{
				SelfID: selfID,
				IDs:    append(append([]string{}, peers[:x]...), peers[x+1:]...), // omit self
				Ready:  true,
			}
			// sendPeers is asynchronous, wait for the signal
			err := receiveWithTimeout(500*time.Millisecond, strategy.forcedResync())
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
					assert.Equal(t, isOwnerExpected, strategy.isOwner(cluster), "Incorrect cluster ownership")
				})
			}
		}
	})

	t.Run("forcedResync channel sends on peer change", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockPeerManager := NewMockPeerManager(ctrl)

		var peersChan chan<- peermanager.Peers
		mockPeerManager.EXPECT().AddListener(gomock.Any()).Do(func(lis chan<- peermanager.Peers) {
			peersChan = lis
		})
		strategy := newPeersBasedStrategy(t.Context(), mockPeerManager, false)
		mockPeerManager.EXPECT().RemoveListener(peersChan).AnyTimes()

		// Initial peers
		peersChan <- peermanager.Peers{
			SelfID: "peer1",
		}
		err := receiveWithTimeout(500*time.Millisecond, strategy.forcedResync())
		assert.NoError(t, err, "Expected resync channel to be sent")

		// Change peers - expect a resync
		peersChan <- peermanager.Peers{
			SelfID: "peer1",
			IDs:    []string{"peer2"},
		}
		err = receiveWithTimeout(500*time.Millisecond, strategy.forcedResync())
		assert.NoError(t, err, "Expected resync channel to be sent")

		// Send same peers again - no resync expected
		peersChan <- peermanager.Peers{
			SelfID: "peer1",
			IDs:    []string{"peer2"},
		}
		err = receiveWithTimeout(500*time.Millisecond, strategy.forcedResync())
		assert.Error(t, err, "Unexpected resync")
	})

	t.Run("cleanup on context cancellation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockPeerManager := NewMockPeerManager(ctrl)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var peersChan chan<- peermanager.Peers
		mockPeerManager.EXPECT().AddListener(gomock.Any()).Do(func(lis chan<- peermanager.Peers) {
			peersChan = lis
		})
		strategy := newPeersBasedStrategy(ctx, mockPeerManager, false)
		mockPeerManager.EXPECT().RemoveListener(peersChan).Times(1)

		// Simulate a single, leader replica
		peersChan <- peermanager.Peers{
			SelfID: "peer1",
			Ready:  true,
			Leader: true,
		}
		err := receiveWithTimeout(500*time.Millisecond, strategy.forcedResync())
		assert.NoError(t, err, "Expected resync channel to be sent")
		assert.True(t, strategy.isOwner(cluster), "Single leader replica should own")

		// Cancel the context
		cancel()

		// Wait a bit for the goroutine to process the cancellation
		time.Sleep(100 * time.Millisecond)

		var open bool
		select {
		case _, open = <-strategy.forcedResync():
		default:
			assert.False(t, open, "Expected forcedResync channel to be closed")
		}
		assert.False(t, open, "Expected forcedResync channel to be closed")
		assert.False(t, strategy.isOwner(cluster), "Should not own after context canceled")
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
		assert.Equal(t, expectedIDs, strategy.getPeers().IDs, "SelfID should be added and IDs sorted")

		peersWithSelf := peermanager.Peers{
			SelfID: "X",
			IDs:    []string{"X", "Y"}, // SelfID is already in IDs
			Ready:  true,
			Leader: true,
		}
		changed := strategy.setPeers(peersWithSelf)
		expectedIDsWithSelf := []string{"X", "Y"}
		assert.Equal(t, expectedIDsWithSelf, strategy.getPeers().IDs, "SelfID should not be duplicated if already present")
		assert.True(t, changed, "Expected setPeers to return true")
	})
}

func TestConsistentHashing_StabilityAndDistribution(t *testing.T) {
	const numClusters = 300
	var clusters []*v3.Cluster
	for i := range numClusters {
		clusters = append(clusters, &v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("cluster-%d", i),
				UID:  uuid.NewUUID(),
			},
		})
	}
	newPeerManager := func(t *testing.T, useConsistent bool, peers peermanager.Peers) ownerStrategy {
		ctrl := gomock.NewController(t)
		pm := NewMockPeerManager(ctrl)

		var peersChan chan<- peermanager.Peers
		pm.EXPECT().AddListener(gomock.Any()).Do(func(lis chan<- peermanager.Peers) {
			peersChan = lis
		})
		s := newPeersBasedStrategy(t.Context(), pm, useConsistent)
		pm.EXPECT().RemoveListener(peersChan).AnyTimes()

		peersChan <- peers
		return s
	}

	// Helper to calculate which peer owns which cluster
	// Returns a map[ClusterUID] -> OwnerID
	getOwnershipMap := func(t *testing.T, peersList []string, useConsistent bool) map[types.UID]string {
		// Create a strategy for each peer to simulate their distributed view
		strategiesByPeer := make(map[string]ownerStrategy)
		for x, peerID := range peersList {
			s := newPeerManager(t, useConsistent, peermanager.Peers{
				SelfID: peerID,
				IDs:    append(append([]string{}, peersList[:x]...), peersList[x+1:]...),
				Ready:  true,
				Leader: x == 0,
			})
			err := receiveWithTimeout(100*time.Millisecond, s.forcedResync())
			assert.NoError(t, err, "Expected resync channel to be sent")

			strategiesByPeer[peerID] = s
		}

		ownership := make(map[types.UID]string)
		for _, c := range clusters {
			var found bool
			for _, peerID := range peersList {
				if strategiesByPeer[peerID].isOwner(c) {
					if found {
						t.Fatalf("Split Brain detected! Cluster %s is claimed by multiple owners (collision at %s)", c.UID, peerID)
					}
					found = true
					ownership[c.UID] = peerID
				}
			}
			if !found {
				t.Fatalf("Orphaned Cluster! Cluster %s has no owner", c.UID)
			}
		}
		return ownership
	}

	calculateChurn := func(before, after map[types.UID]string) int {
		moves := 0
		for uid, owner := range before {
			if after[uid] != owner {
				moves++
			}
		}
		return moves
	}

	t.Run("Scenario: Peer Failure (Scale Down 3->2)", func(t *testing.T) {
		initialPeers := []string{"peer-A", "peer-B", "peer-C"}
		remainingPeers := []string{"peer-B", "peer-C"} // peer-A dies (worst case in the legacy approach)

		legacyMoves := calculateChurn(
			getOwnershipMap(t, initialPeers, false),
			getOwnershipMap(t, remainingPeers, false),
		)
		consistentMoves := calculateChurn(
			getOwnershipMap(t, initialPeers, true),
			getOwnershipMap(t, remainingPeers, true),
		)

		t.Logf("Legacy Churn: %d/%d moved (%d%%)", legacyMoves, numClusters, 100*legacyMoves/numClusters)
		t.Logf("Consistent Churn: %d/%d moved (%d%%)", consistentMoves, numClusters, 100*consistentMoves/numClusters)

		// Assertion 1: Legacy behaves poorly when the first peer is removed (Index Shift)
		// Math: Removing 'peer-A' (index 0) causes a domino effect in Range Partitioning.
		// 1. peer-A's load (~33%) moves to B.
		// 2. peer-B's original load (~33%) shifts indices, forcing half of it (~16%) to move to C.
		// Total expected churn: ~33% + ~16% = ~50%.
		assert.Greater(t, 100*legacyMoves/numClusters, 40, "Legacy approach should have high churn (>40%%)")

		// Assertion 2: Consistent behaves ideally (only A's items move)
		// Math: peer-A owned ~1/3 (approx 333 items). Only those should move.
		// We expect churn to be close to 33%.
		assert.True(t, consistentMoves > 0, "Consistent approach should still move dead peer's items")
		assert.Less(t, 100*consistentMoves/numClusters, 40, "Consistent approach should have low churn (<40%%)")
	})

	t.Run("Scenario: Rolling Update (Replacement A -> D)", func(t *testing.T) {
		// This is the worst situation for Range Partitioning.
		// The first peer is replaced ("peer-A") with a last peer ("peer-D").
		// Index 0 changes from A to B. Index 1 from B to C => complete reshuffle
		oldPeers := []string{"peer-A", "peer-B", "peer-C"}
		newPeers := []string{"peer-B", "peer-C", "peer-D"}

		legacyMoves := calculateChurn(
			getOwnershipMap(t, oldPeers, false),
			getOwnershipMap(t, newPeers, false),
		)
		consistentMoves := calculateChurn(
			getOwnershipMap(t, oldPeers, true),
			getOwnershipMap(t, newPeers, true),
		)

		t.Logf("Legacy Moves: %d (%d%%)", legacyMoves, 100*legacyMoves/numClusters)
		t.Logf("Consistent Moves: %d (%d%%)", consistentMoves, 100*consistentMoves/numClusters)

		// Legacy: Indices shift for EVERYONE.
		// peer-B moves from Index 1 to Index 0. It drops its old keys and takes new ones.
		assert.Greater(t, 100*legacyMoves/numClusters, 75, "Legacy should effectively reshuffle everything (>75%)")

		// Consistent:
		// 1. peer-A leaves: Its 33% must move.
		// 2. peer-D joins: It steals ~33% of the TOTAL cluster from others.
		// Overlap: D will take some of A's old work, and some of B/C's work.
		// B and C keep whatever D didn't steal.
		// Expected: ~40-50% churn (much better than 80-100%).
		assert.Less(t, 100*consistentMoves/numClusters, 55, "Consistent should be significantly more stable than Legacy")
	})

	t.Run("Scenario: Scale Up (3->4)", func(t *testing.T) {
		peers3 := []string{"peer-A", "peer-B", "peer-C"}
		peers4 := []string{"peer-A", "peer-B", "peer-C", "peer-D"}

		legacyMoves := calculateChurn(
			getOwnershipMap(t, peers3, false),
			getOwnershipMap(t, peers4, false),
		)
		consistentMoves := calculateChurn(
			getOwnershipMap(t, peers3, true),
			getOwnershipMap(t, peers4, true),
		)

		t.Logf("Legacy Moves: %d (%d%%)", legacyMoves, 100*legacyMoves/numClusters)
		t.Logf("Consistent Moves: %d (%d%%)", consistentMoves, 100*consistentMoves/numClusters)

		// Legacy: N changes from 3 to 4. The division factor changes.
		// Every range boundary shifts.
		// Expected: > 50% churn.
		assert.Greater(t, 100*legacyMoves/numClusters, 0, "Legacy churn should be high on scale up (>50%)")

		// Consistent: peer-D simply takes 1/4th of the items.
		// Expected: ~25% churn.
		assert.Less(t, 100*consistentMoves/numClusters, 30, "Consistent churn should match 1/N (approx 25%)")
	})
	t.Run("Scenario: Distribution Uniformity", func(t *testing.T) {
		peers := []string{"peer-A", "peer-B", "peer-C"}
		ownership := getOwnershipMap(t, peers, true)
		counts := countByValue(ownership)

		// We assert distribution is not terribly unbalanced
		for peer, count := range counts {
			t.Logf("Peer %s owns %d clusters", peer, count)
			assert.InDelta(t, 100*count/numClusters, 33, 5, "Distribution is too sparse for peer %s (ideal ~33%)", peer)
		}
	})
}

func countByValue[K, V comparable](m map[K]V) map[V]int {
	counts := make(map[V]int)
	for _, v := range m {
		counts[v]++
	}
	return counts
}

func receiveWithTimeout[T any](timeout time.Duration, ch <-chan T) error {
	select {
	case <-ch:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("Timed out waiting for channel")
	}
}
