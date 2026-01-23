package usercontrollers

import (
	"context"
	"hash/crc32"
	"math"
	"slices"
	"sync"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/metrics"
	"github.com/rancher/rancher/pkg/peermanager"
	"github.com/sirupsen/logrus"
)

type ownerStrategy interface {
	// isOwner returns true if the current process owns the provided downstream Cluster
	isOwner(cluster *v3.Cluster) bool
	// forcedResync provides a channel to communicate events that may require a resync in the consumer
	// it could be nil for some implementations
	forcedResync() <-chan struct{}
}

func getOwnerStrategy(ctx context.Context, m peermanager.PeerManager) ownerStrategy {
	if m == nil {
		return &nonClusteredStrategy{}
	}
	return newPeersBasedStrategy(ctx, m)
}

// nonClusteredStrategy makes a single Rancher replica the owner of every downstream cluster
type nonClusteredStrategy struct{}

func (nonClusteredStrategy) forcedResync() <-chan struct{} {
	return nil
}

func (nonClusteredStrategy) isOwner(_ *v3.Cluster) bool {
	return true
}

// peersBasedStrategy uses peers information to decide the owners for every downstream cluster
type peersBasedStrategy struct {
	forcedResyncChan chan struct{}

	// mu protects peers for concurrent read/write access
	mu    sync.Mutex
	peers peermanager.Peers
}

func (s *peersBasedStrategy) forcedResync() <-chan struct{} {
	return s.forcedResyncChan
}

func (s *peersBasedStrategy) isOwner(cluster *v3.Cluster) (owner bool) {
	peers := s.getPeers()
	if peers.SelfID == "" {
		// not ready
		return false
	}
	defer func() {
		if owner {
			metrics.SetClusterOwner(peers.SelfID, cluster.Name)
		} else {
			metrics.UnsetClusterOwner(peers.SelfID, cluster.Name)
		}
	}()

	// Possible assumption on this condition:
	// - peers.IDs with just 1 item will be just SelfID (IDs should never be empty, but better use caution)
	// - then, being a sole non-leader replica, should not own any downstream until becoming the leader
	if !peers.Ready || len(peers.IDs) == 0 || (len(peers.IDs) == 1 && !peers.Leader) {
		return false
	}

	ck := crc32.ChecksumIEEE([]byte(cluster.UID))
	if ck == math.MaxUint32 {
		ck--
	}

	// This math needs 64-bit size to be safe, as "ck" is uint32 and will be multiplied by the number of peers
	// int is equivalent to int64 in 64-bit systems, but better be explicit
	scaled := uint64(ck) * uint64(len(peers.IDs)) / math.MaxUint32
	logrus.Debugf("%s(%v): (%v * %v) / %v = %v[%v] = %v, self = %v\n", cluster.Name, cluster.UID, ck,
		uint32(len(peers.IDs)), math.MaxUint32, peers.IDs, scaled, peers.IDs[scaled], peers.SelfID)
	return peers.IDs[scaled] == peers.SelfID
}

func (s *peersBasedStrategy) getPeers() peermanager.Peers {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.peers
}

func (s *peersBasedStrategy) setPeers(peers peermanager.Peers) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := append(slices.Clone(peers.IDs), peers.SelfID)
	slices.Sort(ids)
	peers.IDs = slices.Compact(ids)

	if s.peers.Equals(peers) {
		return false
	}

	s.peers = peers
	return true
}

func (s *peersBasedStrategy) triggerForcedResync() {
	// try-send to channel, combined with a 1-sized buffer, allows aggregating successive signals while the consumer may still be processing a previous one
	select {
	case s.forcedResyncChan <- struct{}{}:
	default:
	}
}

func newPeersBasedStrategy(ctx context.Context, m peermanager.PeerManager) *peersBasedStrategy {
	// PeerManager watches Endpoints in the "rancher" Service to detect available pods, sending peer updates
	peersChan := make(chan peermanager.Peers, 100)
	m.AddListener(peersChan)

	forcedResync := make(chan struct{}, 1)
	s := &peersBasedStrategy{
		forcedResyncChan: forcedResync,
	}

	go func() {
		defer close(forcedResync)
		for {
			select {
			// Keep remotedialer peers list up to date, triggering a resync of ownership when they change
			case peers := <-peersChan:
				if s.setPeers(peers) {
					s.triggerForcedResync()
				}
			case <-ctx.Done():
				m.RemoveListener(peersChan)
				close(peersChan)
				//revive:disable:empty-block Until https://github.com/mgechev/revive/issues/386 is fixed
				for range peersChan {
					// drain channel
				}
				//revive:enable:empty-block
				return
			}
		}
	}()

	return s
}
