package client

const (
	AgentDeploymentCustomizationType                              = "agentDeploymentCustomization"
	AgentDeploymentCustomizationFieldAppendTolerations            = "appendTolerations"
	AgentDeploymentCustomizationFieldOverrideAffinity             = "overrideAffinity"
	AgentDeploymentCustomizationFieldOverrideResourceRequirements = "overrideResourceRequirements"
)

type AgentDeploymentCustomization struct {
	AppendTolerations            []Toleration          `json:"appendTolerations,omitempty" yaml:"appendTolerations,omitempty"`
	OverrideAffinity             *Affinity             `json:"overrideAffinity,omitempty" yaml:"overrideAffinity,omitempty"`
	OverrideResourceRequirements *ResourceRequirements `json:"overrideResourceRequirements,omitempty" yaml:"overrideResourceRequirements,omitempty"`
}
