package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	StatefulSetUpdateStrategyType                = "statefulSetUpdateStrategy"
	StatefulSetUpdateStrategyFieldMaxUnavailable = "maxUnavailable"
	StatefulSetUpdateStrategyFieldPartition      = "partition"
	StatefulSetUpdateStrategyFieldStrategy       = "strategy"
)

type StatefulSetUpdateStrategy struct {
	MaxUnavailable intstr.IntOrString `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
	Partition      *int64             `json:"partition,omitempty" yaml:"partition,omitempty"`
	Strategy       string             `json:"strategy,omitempty" yaml:"strategy,omitempty"`
}
