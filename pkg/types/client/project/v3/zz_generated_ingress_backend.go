package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	IngressBackendType             = "ingressBackend"
	IngressBackendFieldResource    = "resource"
	IngressBackendFieldServiceID   = "serviceId"
	IngressBackendFieldTargetPort  = "targetPort"
	IngressBackendFieldWorkloadIDs = "workloadIds"
)

type IngressBackend struct {
	Resource    *TypedLocalObjectReference `json:"resource,omitempty" yaml:"resource,omitempty"`
	ServiceID   string                     `json:"serviceId,omitempty" yaml:"serviceId,omitempty"`
	TargetPort  intstr.IntOrString         `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
	WorkloadIDs []string                   `json:"workloadIds,omitempty" yaml:"workloadIds,omitempty"`
}
