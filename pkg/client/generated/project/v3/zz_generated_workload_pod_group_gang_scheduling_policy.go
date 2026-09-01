package client

const (
	WorkloadPodGroupGangSchedulingPolicyType          = "workloadPodGroupGangSchedulingPolicy"
	WorkloadPodGroupGangSchedulingPolicyFieldMinCount = "minCount"
)

type WorkloadPodGroupGangSchedulingPolicy struct {
	MinCount *int64 `json:"minCount,omitempty" yaml:"minCount,omitempty"`
}
