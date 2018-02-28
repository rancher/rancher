package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	HTTPIngressPathType             = "httpIngressPath"
	HTTPIngressPathFieldServiceId   = "serviceId"
	HTTPIngressPathFieldTargetPort  = "targetPort"
	HTTPIngressPathFieldWorkloadIDs = "workloadIds"
)

type HTTPIngressPath struct {
	ServiceId   string             `json:"serviceId,omitempty" yaml:"serviceId,omitempty"`
	TargetPort  intstr.IntOrString `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
	WorkloadIDs []string           `json:"workloadIds,omitempty" yaml:"workloadIds,omitempty"`
}
