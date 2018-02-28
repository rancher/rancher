package client

const (
	FSGroupStrategyOptionsType        = "fsGroupStrategyOptions"
	FSGroupStrategyOptionsFieldRanges = "ranges"
	FSGroupStrategyOptionsFieldRule   = "rule"
)

type FSGroupStrategyOptions struct {
	Ranges []IDRange `json:"ranges,omitempty" yaml:"ranges,omitempty"`
	Rule   string    `json:"rule,omitempty" yaml:"rule,omitempty"`
}
