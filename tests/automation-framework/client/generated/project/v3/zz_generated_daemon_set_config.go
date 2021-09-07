package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	DaemonSetConfigType                      = "daemonSetConfig"
	DaemonSetConfigFieldMaxUnavailable       = "maxUnavailable"
	DaemonSetConfigFieldMinReadySeconds      = "minReadySeconds"
	DaemonSetConfigFieldRevisionHistoryLimit = "revisionHistoryLimit"
	DaemonSetConfigFieldStrategy             = "strategy"
)

type DaemonSetConfig struct {
	MaxUnavailable       intstr.IntOrString `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
	MinReadySeconds      int64              `json:"minReadySeconds,omitempty" yaml:"minReadySeconds,omitempty"`
	RevisionHistoryLimit *int64             `json:"revisionHistoryLimit,omitempty" yaml:"revisionHistoryLimit,omitempty"`
	Strategy             string             `json:"strategy,omitempty" yaml:"strategy,omitempty"`
}
