package client

const (
	WorkloadReferenceType                    = "workloadReference"
	WorkloadReferenceFieldName               = "name"
	WorkloadReferenceFieldPodGroup           = "podGroup"
	WorkloadReferenceFieldPodGroupReplicaKey = "podGroupReplicaKey"
)

type WorkloadReference struct {
	Name               string `json:"name,omitempty" yaml:"name,omitempty"`
	PodGroup           string `json:"podGroup,omitempty" yaml:"podGroup,omitempty"`
	PodGroupReplicaKey string `json:"podGroupReplicaKey,omitempty" yaml:"podGroupReplicaKey,omitempty"`
}
