package client

const (
	NodeDriverStatusType            = "nodeDriverStatus"
	NodeDriverStatusFieldConditions = "conditions"
)

type NodeDriverStatus struct {
	Conditions []NodeDriverCondition `json:"conditions,omitempty"`
}
