package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	RollingUpdateStatefulSetStrategyType                = "rollingUpdateStatefulSetStrategy"
	RollingUpdateStatefulSetStrategyFieldMaxUnavailable = "maxUnavailable"
	RollingUpdateStatefulSetStrategyFieldPartition      = "partition"
)

type RollingUpdateStatefulSetStrategy struct {
	MaxUnavailable intstr.IntOrString `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
	Partition      *int64             `json:"partition,omitempty" yaml:"partition,omitempty"`
}
