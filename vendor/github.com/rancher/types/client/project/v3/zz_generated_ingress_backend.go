package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	IngressBackendType             = "ingressBackend"
	IngressBackendFieldServiceId   = "serviceId"
	IngressBackendFieldTargetPort  = "targetPort"
	IngressBackendFieldWorkloadIDs = "workloadIds"
)

type IngressBackend struct {
	ServiceId   string             `json:"serviceId,omitempty"`
	TargetPort  intstr.IntOrString `json:"targetPort,omitempty"`
	WorkloadIDs []string           `json:"workloadIds,omitempty"`
}
