package client

const (
	PriorityClassSpecType            = "priorityClassSpec"
	PriorityClassSpecFieldPreemption = "preemption"
	PriorityClassSpecFieldValue      = "value"
)

type PriorityClassSpec struct {
	Preemption string `json:"preemption,omitempty" yaml:"preemption,omitempty"`
	Value      int64  `json:"value,omitempty" yaml:"value,omitempty"`
}
