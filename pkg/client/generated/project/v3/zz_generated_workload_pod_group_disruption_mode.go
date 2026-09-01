package client

const (
	WorkloadPodGroupDisruptionModeType        = "workloadPodGroupDisruptionMode"
	WorkloadPodGroupDisruptionModeFieldAll    = "all"
	WorkloadPodGroupDisruptionModeFieldSingle = "single"
)

type WorkloadPodGroupDisruptionMode struct {
	All    *WorkloadPodGroupAllDisruptionMode    `json:"all,omitempty" yaml:"all,omitempty"`
	Single *WorkloadPodGroupSingleDisruptionMode `json:"single,omitempty" yaml:"single,omitempty"`
}
