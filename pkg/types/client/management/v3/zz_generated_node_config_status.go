package client

const (
	NodeConfigStatusType               = "nodeConfigStatus"
	NodeConfigStatusFieldActive        = "active"
	NodeConfigStatusFieldAssigned      = "assigned"
	NodeConfigStatusFieldError         = "error"
	NodeConfigStatusFieldLastKnownGood = "lastKnownGood"
)

type NodeConfigStatus struct {
	Active        *NodeConfigSource `json:"active,omitempty" yaml:"active,omitempty"`
	Assigned      *NodeConfigSource `json:"assigned,omitempty" yaml:"assigned,omitempty"`
	Error         string            `json:"error,omitempty" yaml:"error,omitempty"`
	LastKnownGood *NodeConfigSource `json:"lastKnownGood,omitempty" yaml:"lastKnownGood,omitempty"`
}
