package client

const (
	NodeSelectorTermType                  = "nodeSelectorTerm"
	NodeSelectorTermFieldMatchExpressions = "matchExpressions"
	NodeSelectorTermFieldMatchFields      = "matchFields"
)

type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement `json:"matchExpressions,omitempty" yaml:"matchExpressions,omitempty"`
	MatchFields      []NodeSelectorRequirement `json:"matchFields,omitempty" yaml:"matchFields,omitempty"`
}
