package client

const (
	PodAffinityTermType               = "podAffinityTerm"
	PodAffinityTermFieldLabelSelector = "labelSelector"
	PodAffinityTermFieldNamespaces    = "namespaces"
	PodAffinityTermFieldTopologyKey   = "topologyKey"
)

type PodAffinityTerm struct {
	LabelSelector *LabelSelector `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
	Namespaces    []string       `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	TopologyKey   string         `json:"topologyKey,omitempty" yaml:"topologyKey,omitempty"`
}
