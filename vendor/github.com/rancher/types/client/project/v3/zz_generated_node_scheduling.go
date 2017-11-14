package client

const (
	NodeSchedulingType            = "nodeScheduling"
	NodeSchedulingFieldName       = "name"
	NodeSchedulingFieldPreferred  = "preferred"
	NodeSchedulingFieldRequireAll = "requireAll"
	NodeSchedulingFieldRequireAny = "requireAny"
)

type NodeScheduling struct {
	Name       string   `json:"name,omitempty"`
	Preferred  []string `json:"preferred,omitempty"`
	RequireAll []string `json:"requireAll,omitempty"`
	RequireAny []string `json:"requireAny,omitempty"`
}
