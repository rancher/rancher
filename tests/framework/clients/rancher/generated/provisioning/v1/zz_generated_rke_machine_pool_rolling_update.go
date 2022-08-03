package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	RKEMachinePoolRollingUpdateType                = "rkeMachinePoolRollingUpdate"
	RKEMachinePoolRollingUpdateFieldMaxSurge       = "maxSurge"
	RKEMachinePoolRollingUpdateFieldMaxUnavailable = "maxUnavailable"
)

type RKEMachinePoolRollingUpdate struct {
	MaxSurge       intstr.IntOrString `json:"maxSurge,omitempty" yaml:"maxSurge,omitempty"`
	MaxUnavailable intstr.IntOrString `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
}
