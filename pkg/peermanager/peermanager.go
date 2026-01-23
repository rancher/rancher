package peermanager

import "slices"

type Peers struct {
	SelfID string
	IDs    []string
	Ready  bool
	Leader bool
}

func (p Peers) Equals(other Peers) bool {
	return p.SelfID == other.SelfID &&
		p.Ready == other.Ready &&
		p.Leader == other.Leader &&
		slices.Equal(p.IDs, other.IDs)
}

type PeerManager interface {
	IsLeader() bool
	Leader()
	AddListener(l chan<- Peers)
	RemoveListener(l chan<- Peers)
}
