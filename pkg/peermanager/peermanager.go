package peermanager

type Peers struct {
	SelfID string
	IDs    []string
	Ready  bool
	Leader bool
}

type PeerManager interface {
	IsLeader() bool
	Leader()
	AddListener(l chan<- Peers)
	RemoveListener(l chan<- Peers)
}
