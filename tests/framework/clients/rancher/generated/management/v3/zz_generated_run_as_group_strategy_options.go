package client

const (
	RunAsGroupStrategyOptionsType        = "runAsGroupStrategyOptions"
	RunAsGroupStrategyOptionsFieldRanges = "ranges"
	RunAsGroupStrategyOptionsFieldRule   = "rule"
)

type RunAsGroupStrategyOptions struct {
	Ranges []IDRange `json:"ranges,omitempty" yaml:"ranges,omitempty"`
	Rule   string    `json:"rule,omitempty" yaml:"rule,omitempty"`
}
