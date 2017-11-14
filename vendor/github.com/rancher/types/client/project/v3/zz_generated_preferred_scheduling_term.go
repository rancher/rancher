package client

const (
	PreferredSchedulingTermType            = "preferredSchedulingTerm"
	PreferredSchedulingTermFieldPreference = "preference"
	PreferredSchedulingTermFieldWeight     = "weight"
)

type PreferredSchedulingTerm struct {
	Preference *NodeSelectorTerm `json:"preference,omitempty"`
	Weight     *int64            `json:"weight,omitempty"`
}
