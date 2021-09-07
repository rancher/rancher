package client

const (
	TopologySelectorTermType                       = "topologySelectorTerm"
	TopologySelectorTermFieldMatchLabelExpressions = "matchLabelExpressions"
)

type TopologySelectorTerm struct {
	MatchLabelExpressions []TopologySelectorLabelRequirement `json:"matchLabelExpressions,omitempty" yaml:"matchLabelExpressions,omitempty"`
}
