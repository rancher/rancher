package client

const (
	WorkloadPodGroupSchedulingConstraintsType          = "workloadPodGroupSchedulingConstraints"
	WorkloadPodGroupSchedulingConstraintsFieldTopology = "topology"
)

type WorkloadPodGroupSchedulingConstraints struct {
	Topology []TopologyConstraint `json:"topology,omitempty" yaml:"topology,omitempty"`
}
