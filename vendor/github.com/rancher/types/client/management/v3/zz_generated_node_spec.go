package client

const (
	NodeSpecType               = "nodeSpec"
	NodeSpecFieldConfigSource  = "configSource"
	NodeSpecFieldExternalId    = "externalId"
	NodeSpecFieldPodCIDR       = "podCIDR"
	NodeSpecFieldProviderID    = "providerID"
	NodeSpecFieldTaints        = "taints"
	NodeSpecFieldUnschedulable = "unschedulable"
)

type NodeSpec struct {
	ConfigSource  *NodeConfigSource `json:"configSource,omitempty"`
	ExternalId    string            `json:"externalId,omitempty"`
	PodCIDR       string            `json:"podCIDR,omitempty"`
	ProviderID    string            `json:"providerID,omitempty"`
	Taints        []Taint           `json:"taints,omitempty"`
	Unschedulable *bool             `json:"unschedulable,omitempty"`
}
