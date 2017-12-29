package client

const (
	NodeSpecType               = "nodeSpec"
	NodeSpecFieldPodCidr       = "podCidr"
	NodeSpecFieldProviderId    = "providerId"
	NodeSpecFieldTaints        = "taints"
	NodeSpecFieldUnschedulable = "unschedulable"
)

type NodeSpec struct {
	PodCidr       string  `json:"podCidr,omitempty"`
	ProviderId    string  `json:"providerId,omitempty"`
	Taints        []Taint `json:"taints,omitempty"`
	Unschedulable *bool   `json:"unschedulable,omitempty"`
}
