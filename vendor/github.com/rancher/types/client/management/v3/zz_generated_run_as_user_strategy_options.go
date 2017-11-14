package client

const (
	RunAsUserStrategyOptionsType        = "runAsUserStrategyOptions"
	RunAsUserStrategyOptionsFieldRanges = "ranges"
	RunAsUserStrategyOptionsFieldRule   = "rule"
)

type RunAsUserStrategyOptions struct {
	Ranges []IDRange `json:"ranges,omitempty"`
	Rule   string    `json:"rule,omitempty"`
}
