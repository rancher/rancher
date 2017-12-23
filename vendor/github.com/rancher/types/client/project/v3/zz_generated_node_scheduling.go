package client

const (
	NodeSchedulingType            = "nodeScheduling"
	NodeSchedulingFieldNodeId     = "nodeId"
	NodeSchedulingFieldPreferred  = "preferred"
	NodeSchedulingFieldRequireAll = "requireAll"
	NodeSchedulingFieldRequireAny = "requireAny"
)

type NodeScheduling struct {
	NodeId     string   `json:"nodeId,omitempty"`
	Preferred  []string `json:"preferred,omitempty"`
	RequireAll []string `json:"requireAll,omitempty"`
	RequireAny []string `json:"requireAny,omitempty"`
}
