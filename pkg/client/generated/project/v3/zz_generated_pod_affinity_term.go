package client

const (
	PodAffinityTermType                   = "podAffinityTerm"
	PodAffinityTermFieldLabelSelector     = "labelSelector"
	PodAffinityTermFieldNamespaceSelector = "namespaceSelector"
	PodAffinityTermFieldNamespaces        = "namespaces"
	PodAffinityTermFieldTopologyKey       = "topologyKey"
)

type PodAffinityTerm struct {
	LabelSelector     *LabelSelector `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
	NamespaceSelector *LabelSelector `json:"namespaceSelector,omitempty" yaml:"namespaceSelector,omitempty"`
	Namespaces        []string       `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	TopologyKey       string         `json:"topologyKey,omitempty" yaml:"topologyKey,omitempty"`
}
