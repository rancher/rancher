package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	HTTPIngressPathType             = "httpIngressPath"
	HTTPIngressPathFieldServiceID   = "serviceId"
	HTTPIngressPathFieldTargetPort  = "targetPort"
	HTTPIngressPathFieldWorkloadIDs = "workloadIds"
)

type HTTPIngressPath struct {
	ServiceID   string             `json:"serviceId,omitempty" yaml:"serviceId,omitempty"`
	TargetPort  intstr.IntOrString `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
	WorkloadIDs []string           `json:"workloadIds,omitempty" yaml:"workloadIds,omitempty"`
}
