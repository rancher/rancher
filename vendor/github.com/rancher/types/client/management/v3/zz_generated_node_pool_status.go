package client

const (
	NodePoolStatusType            = "nodePoolStatus"
	NodePoolStatusFieldConditions = "conditions"
)

type NodePoolStatus struct {
	Conditions []Condition `json:"conditions,omitempty"`
}
