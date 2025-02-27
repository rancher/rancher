package client

const (
	PriorityClassSpecType                  = "priorityClassSpec"
	PriorityClassSpecFieldPreemptionPolicy = "preemptionPolicy"
	PriorityClassSpecFieldValue            = "value"
)

type PriorityClassSpec struct {
	PreemptionPolicy string `json:"preemptionPolicy,omitempty" yaml:"preemptionPolicy,omitempty"`
	Value            int64  `json:"value,omitempty" yaml:"value,omitempty"`
}
