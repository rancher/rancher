package client

const (
	SchedulingType                   = "scheduling"
	SchedulingFieldNode              = "node"
	SchedulingFieldPriority          = "priority"
	SchedulingFieldPriorityClassName = "priorityClassName"
	SchedulingFieldScheduler         = "scheduler"
	SchedulingFieldTolerate          = "tolerate"
)

type Scheduling struct {
	Node              *NodeScheduling `json:"node,omitempty" yaml:"node,omitempty"`
	Priority          *int64          `json:"priority,omitempty" yaml:"priority,omitempty"`
	PriorityClassName string          `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	Scheduler         string          `json:"scheduler,omitempty" yaml:"scheduler,omitempty"`
	Tolerate          []Toleration    `json:"tolerate,omitempty" yaml:"tolerate,omitempty"`
}
