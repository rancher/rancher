package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	RollingUpdateDaemonSetType                = "rollingUpdateDaemonSet"
	RollingUpdateDaemonSetFieldMaxSurge       = "maxSurge"
	RollingUpdateDaemonSetFieldMaxUnavailable = "maxUnavailable"
)

type RollingUpdateDaemonSet struct {
	MaxSurge       intstr.IntOrString `json:"maxSurge,omitempty" yaml:"maxSurge,omitempty"`
	MaxUnavailable intstr.IntOrString `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
}
