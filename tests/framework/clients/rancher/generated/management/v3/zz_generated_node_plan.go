package client

const (
	NodePlanType                    = "nodePlan"
	NodePlanFieldAgentCheckInterval = "agentCheckInterval"
	NodePlanFieldPlan               = "plan"
	NodePlanFieldVersion            = "version"
)

type NodePlan struct {
	AgentCheckInterval int64              `json:"agentCheckInterval,omitempty" yaml:"agentCheckInterval,omitempty"`
	Plan               *RKEConfigNodePlan `json:"plan,omitempty" yaml:"plan,omitempty"`
	Version            int64              `json:"version,omitempty" yaml:"version,omitempty"`
}
