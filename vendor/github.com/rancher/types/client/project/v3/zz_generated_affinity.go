package client

const (
	AffinityType                 = "affinity"
	AffinityFieldNodeAffinity    = "nodeAffinity"
	AffinityFieldPodAffinity     = "podAffinity"
	AffinityFieldPodAntiAffinity = "podAntiAffinity"
)

type Affinity struct {
	NodeAffinity    *NodeAffinity    `json:"nodeAffinity,omitempty"`
	PodAffinity     *PodAffinity     `json:"podAffinity,omitempty"`
	PodAntiAffinity *PodAntiAffinity `json:"podAntiAffinity,omitempty"`
}
