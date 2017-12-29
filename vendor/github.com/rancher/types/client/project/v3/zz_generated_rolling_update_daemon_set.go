package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	RollingUpdateDaemonSetType                = "rollingUpdateDaemonSet"
	RollingUpdateDaemonSetFieldMaxUnavailable = "maxUnavailable"
)

type RollingUpdateDaemonSet struct {
	MaxUnavailable intstr.IntOrString `json:"maxUnavailable,omitempty"`
}
