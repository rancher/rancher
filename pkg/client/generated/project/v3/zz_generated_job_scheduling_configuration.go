package client

const (
	JobSchedulingConfigurationType                       = "jobSchedulingConfiguration"
	JobSchedulingConfigurationFieldDisruptionMode        = "disruptionMode"
	JobSchedulingConfigurationFieldResourceClaims        = "resourceClaims"
	JobSchedulingConfigurationFieldSchedulingConstraints = "schedulingConstraints"
	JobSchedulingConfigurationFieldSchedulingPolicy      = "schedulingPolicy"
)

type JobSchedulingConfiguration struct {
	DisruptionMode        *WorkloadPodGroupDisruptionMode        `json:"disruptionMode,omitempty" yaml:"disruptionMode,omitempty"`
	ResourceClaims        []WorkloadPodGroupResourceClaim        `json:"resourceClaims,omitempty" yaml:"resourceClaims,omitempty"`
	SchedulingConstraints *WorkloadPodGroupSchedulingConstraints `json:"schedulingConstraints,omitempty" yaml:"schedulingConstraints,omitempty"`
	SchedulingPolicy      *WorkloadPodGroupSchedulingPolicy      `json:"schedulingPolicy,omitempty" yaml:"schedulingPolicy,omitempty"`
}
