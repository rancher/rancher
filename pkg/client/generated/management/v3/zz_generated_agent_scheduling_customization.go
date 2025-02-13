package client

const (
	AgentSchedulingCustomizationType                     = "agentSchedulingCustomization"
	AgentSchedulingCustomizationFieldPodDisruptionBudget = "podDisruptionBudget"
	AgentSchedulingCustomizationFieldPriorityClass       = "priorityClass"
)

type AgentSchedulingCustomization struct {
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty" yaml:"podDisruptionBudget,omitempty"`
	PriorityClass       *PriorityClassSpec       `json:"priorityClass,omitempty" yaml:"priorityClass,omitempty"`
}
