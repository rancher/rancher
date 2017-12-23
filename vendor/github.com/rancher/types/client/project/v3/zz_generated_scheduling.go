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
	Node              *NodeScheduling `json:"node,omitempty"`
	Priority          *int64          `json:"priority,omitempty"`
	PriorityClassName string          `json:"priorityClassName,omitempty"`
	Scheduler         string          `json:"scheduler,omitempty"`
	Tolerate          []Toleration    `json:"tolerate,omitempty"`
}
