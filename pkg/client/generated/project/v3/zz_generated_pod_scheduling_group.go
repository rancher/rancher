package client

const (
	PodSchedulingGroupType              = "podSchedulingGroup"
	PodSchedulingGroupFieldPodGroupName = "podGroupName"
)

type PodSchedulingGroup struct {
	PodGroupName string `json:"podGroupName,omitempty" yaml:"podGroupName,omitempty"`
}
