package client

const (
	WebhookDeploymentCustomizationType                              = "webhookDeploymentCustomization"
	WebhookDeploymentCustomizationFieldAppendTolerations            = "appendTolerations"
	WebhookDeploymentCustomizationFieldOverrideAffinity             = "overrideAffinity"
	WebhookDeploymentCustomizationFieldOverrideResourceRequirements = "overrideResourceRequirements"
	WebhookDeploymentCustomizationFieldPodDisruptionBudget          = "podDisruptionBudget"
	WebhookDeploymentCustomizationFieldReplicaCount                 = "replicaCount"
)

type WebhookDeploymentCustomization struct {
	AppendTolerations            []Toleration             `json:"appendTolerations,omitempty" yaml:"appendTolerations,omitempty"`
	OverrideAffinity             *Affinity                `json:"overrideAffinity,omitempty" yaml:"overrideAffinity,omitempty"`
	OverrideResourceRequirements *ResourceRequirements    `json:"overrideResourceRequirements,omitempty" yaml:"overrideResourceRequirements,omitempty"`
	PodDisruptionBudget          *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty" yaml:"podDisruptionBudget,omitempty"`
	ReplicaCount                 *int64                   `json:"replicaCount,omitempty" yaml:"replicaCount,omitempty"`
}
