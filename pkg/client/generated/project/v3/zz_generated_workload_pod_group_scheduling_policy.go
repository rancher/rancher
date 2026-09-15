package client

const (
	WorkloadPodGroupSchedulingPolicyType       = "workloadPodGroupSchedulingPolicy"
	WorkloadPodGroupSchedulingPolicyFieldBasic = "basic"
	WorkloadPodGroupSchedulingPolicyFieldGang  = "gang"
)

type WorkloadPodGroupSchedulingPolicy struct {
	Basic *WorkloadPodGroupBasicSchedulingPolicy `json:"basic,omitempty" yaml:"basic,omitempty"`
	Gang  *WorkloadPodGroupGangSchedulingPolicy  `json:"gang,omitempty" yaml:"gang,omitempty"`
}
