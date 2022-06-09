package client

const (
	ComposeStatusType            = "composeStatus"
	ComposeStatusFieldConditions = "conditions"
)

type ComposeStatus struct {
	Conditions []ComposeCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
