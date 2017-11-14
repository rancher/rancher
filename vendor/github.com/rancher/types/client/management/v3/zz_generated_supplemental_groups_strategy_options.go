package client

const (
	SupplementalGroupsStrategyOptionsType        = "supplementalGroupsStrategyOptions"
	SupplementalGroupsStrategyOptionsFieldRanges = "ranges"
	SupplementalGroupsStrategyOptionsFieldRule   = "rule"
)

type SupplementalGroupsStrategyOptions struct {
	Ranges []IDRange `json:"ranges,omitempty"`
	Rule   string    `json:"rule,omitempty"`
}
