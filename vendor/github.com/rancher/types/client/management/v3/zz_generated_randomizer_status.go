package client

const (
	RandomizerStatusType            = "randomizerStatus"
	RandomizerStatusFieldConditions = "conditions"
)

type RandomizerStatus struct {
	Conditions []RandomCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
