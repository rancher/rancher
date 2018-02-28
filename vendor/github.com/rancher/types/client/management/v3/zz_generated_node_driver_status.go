package client

const (
	NodeDriverStatusType            = "nodeDriverStatus"
	NodeDriverStatusFieldConditions = "conditions"
)

type NodeDriverStatus struct {
	Conditions []Condition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
