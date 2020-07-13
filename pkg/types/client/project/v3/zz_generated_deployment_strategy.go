package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	DeploymentStrategyType                = "deploymentStrategy"
	DeploymentStrategyFieldMaxSurge       = "maxSurge"
	DeploymentStrategyFieldMaxUnavailable = "maxUnavailable"
	DeploymentStrategyFieldStrategy       = "strategy"
)

type DeploymentStrategy struct {
	MaxSurge       intstr.IntOrString `json:"maxSurge,omitempty" yaml:"maxSurge,omitempty"`
	MaxUnavailable intstr.IntOrString `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
	Strategy       string             `json:"strategy,omitempty" yaml:"strategy,omitempty"`
}
