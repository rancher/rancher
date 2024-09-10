package client

const (
	NodeSelectorType                   = "nodeSelector"
	NodeSelectorFieldNodeSelectorTerms = "nodeSelectorTerms"
)

type NodeSelector struct {
	NodeSelectorTerms []NodeSelectorTerm `json:"nodeSelectorTerms,omitempty" yaml:"nodeSelectorTerms,omitempty"`
}
