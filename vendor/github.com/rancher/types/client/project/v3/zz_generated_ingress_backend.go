package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	IngressBackendType             = "ingressBackend"
	IngressBackendFieldServiceID   = "serviceId"
	IngressBackendFieldTargetPort  = "targetPort"
	IngressBackendFieldWorkloadIDs = "workloadIds"
)

type IngressBackend struct {
	ServiceID   string             `json:"serviceId,omitempty" yaml:"serviceId,omitempty"`
	TargetPort  intstr.IntOrString `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
	WorkloadIDs []string           `json:"workloadIds,omitempty" yaml:"workloadIds,omitempty"`
}
