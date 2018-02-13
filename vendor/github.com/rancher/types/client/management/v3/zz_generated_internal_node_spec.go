package client

const (
	InternalNodeSpecType               = "internalNodeSpec"
	InternalNodeSpecFieldPodCidr       = "podCidr"
	InternalNodeSpecFieldProviderId    = "providerId"
	InternalNodeSpecFieldTaints        = "taints"
	InternalNodeSpecFieldUnschedulable = "unschedulable"
)

type InternalNodeSpec struct {
	PodCidr       string  `json:"podCidr,omitempty"`
	ProviderId    string  `json:"providerId,omitempty"`
	Taints        []Taint `json:"taints,omitempty"`
	Unschedulable *bool   `json:"unschedulable,omitempty"`
}
