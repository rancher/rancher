package client

const (
	SchedulingType                   = "scheduling"
	SchedulingFieldAntiAffinity      = "antiAffinity"
	SchedulingFieldNode              = "node"
	SchedulingFieldPriority          = "priority"
	SchedulingFieldPriorityClassName = "priorityClassName"
	SchedulingFieldScheduler         = "scheduler"
	SchedulingFieldTolerate          = "tolerate"
)

type Scheduling struct {
	AntiAffinity      string          `json:"antiAffinity,omitempty"`
	Node              *NodeScheduling `json:"node,omitempty"`
	Priority          *int64          `json:"priority,omitempty"`
	PriorityClassName string          `json:"priorityClassName,omitempty"`
	Scheduler         string          `json:"scheduler,omitempty"`
	Tolerate          []string        `json:"tolerate,omitempty"`
}
