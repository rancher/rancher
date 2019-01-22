package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	HTTPIngressPathType             = "httpIngressPath"
	HTTPIngressPathFieldPath        = "path"
	HTTPIngressPathFieldServiceID   = "serviceId"
	HTTPIngressPathFieldTargetPort  = "targetPort"
	HTTPIngressPathFieldWorkloadIDs = "workloadIds"
)

type HTTPIngressPath struct {
	Path        string             `json:"path,omitempty" yaml:"path,omitempty"`
	ServiceID   string             `json:"serviceId,omitempty" yaml:"serviceId,omitempty"`
	TargetPort  intstr.IntOrString `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
	WorkloadIDs []string           `json:"workloadIds,omitempty" yaml:"workloadIds,omitempty"`
}
