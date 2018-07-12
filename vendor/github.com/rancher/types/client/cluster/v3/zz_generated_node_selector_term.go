package client

const (
	NodeSelectorTermType                  = "nodeSelectorTerm"
	NodeSelectorTermFieldMatchExpressions = "matchExpressions"
)

type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement `json:"matchExpressions,omitempty" yaml:"matchExpressions,omitempty"`
}
