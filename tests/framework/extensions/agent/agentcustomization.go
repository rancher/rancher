package agent

import (
	corev1 "k8s.io/api/core/v1"
)

type AgentDeploymentCustomization struct {
	Tolerations          corev1.Toleration           `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	Affinity             corev1.Affinity             `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	ResourceRequirements corev1.ResourceRequirements `json:"resourceRequirements,omitempty" yaml:"resourceRequirements,omitempty"`
}
