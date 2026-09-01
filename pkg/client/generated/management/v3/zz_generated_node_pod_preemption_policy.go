package client

const (
	NodePodPreemptionPolicyType                         = "nodePodPreemptionPolicy"
	NodePodPreemptionPolicyFieldDisableResizePreemption = "disableResizePreemption"
)

type NodePodPreemptionPolicy struct {
	DisableResizePreemption []string `json:"disableResizePreemption,omitempty" yaml:"disableResizePreemption,omitempty"`
}
