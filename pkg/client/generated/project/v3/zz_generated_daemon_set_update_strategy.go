package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	DaemonSetUpdateStrategyType                = "daemonSetUpdateStrategy"
	DaemonSetUpdateStrategyFieldMaxSurge       = "maxSurge"
	DaemonSetUpdateStrategyFieldMaxUnavailable = "maxUnavailable"
	DaemonSetUpdateStrategyFieldStrategy       = "strategy"
)

type DaemonSetUpdateStrategy struct {
	MaxSurge       intstr.IntOrString `json:"maxSurge,omitempty" yaml:"maxSurge,omitempty"`
	MaxUnavailable intstr.IntOrString `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
	Strategy       string             `json:"strategy,omitempty" yaml:"strategy,omitempty"`
}
