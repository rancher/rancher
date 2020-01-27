package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	RollingUpdateStrategyType                = "rollingUpdateStrategy"
	RollingUpdateStrategyFieldDrain          = "drain"
	RollingUpdateStrategyFieldDrainInput     = "nodeDrainInput"
	RollingUpdateStrategyFieldMaxUnavailable = "maxUnavailable"
)

type RollingUpdateStrategy struct {
	Drain          bool               `json:"drain,omitempty" yaml:"drain,omitempty"`
	DrainInput     *NodeDrainInput    `json:"nodeDrainInput,omitempty" yaml:"nodeDrainInput,omitempty"`
	MaxUnavailable intstr.IntOrString `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
}
