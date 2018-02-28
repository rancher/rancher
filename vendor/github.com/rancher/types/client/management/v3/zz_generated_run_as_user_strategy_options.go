package client

const (
	RunAsUserStrategyOptionsType        = "runAsUserStrategyOptions"
	RunAsUserStrategyOptionsFieldRanges = "ranges"
	RunAsUserStrategyOptionsFieldRule   = "rule"
)

type RunAsUserStrategyOptions struct {
	Ranges []IDRange `json:"ranges,omitempty" yaml:"ranges,omitempty"`
	Rule   string    `json:"rule,omitempty" yaml:"rule,omitempty"`
}
