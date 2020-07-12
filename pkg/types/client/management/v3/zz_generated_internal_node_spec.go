package client

const (
	InternalNodeSpecType               = "internalNodeSpec"
	InternalNodeSpecFieldPodCidr       = "podCidr"
	InternalNodeSpecFieldPodCidrs      = "podCidrs"
	InternalNodeSpecFieldProviderId    = "providerId"
	InternalNodeSpecFieldTaints        = "taints"
	InternalNodeSpecFieldUnschedulable = "unschedulable"
)

type InternalNodeSpec struct {
	PodCidr       string   `json:"podCidr,omitempty" yaml:"podCidr,omitempty"`
	PodCidrs      []string `json:"podCidrs,omitempty" yaml:"podCidrs,omitempty"`
	ProviderId    string   `json:"providerId,omitempty" yaml:"providerId,omitempty"`
	Taints        []Taint  `json:"taints,omitempty" yaml:"taints,omitempty"`
	Unschedulable bool     `json:"unschedulable,omitempty" yaml:"unschedulable,omitempty"`
}
