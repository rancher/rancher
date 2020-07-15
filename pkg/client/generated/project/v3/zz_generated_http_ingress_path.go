package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	HTTPIngressPathType             = "httpIngressPath"
	HTTPIngressPathFieldPath        = "path"
	HTTPIngressPathFieldPathType    = "pathType"
	HTTPIngressPathFieldResource    = "resource"
	HTTPIngressPathFieldServiceID   = "serviceId"
	HTTPIngressPathFieldTargetPort  = "targetPort"
	HTTPIngressPathFieldWorkloadIDs = "workloadIds"
)

type HTTPIngressPath struct {
	Path        string                     `json:"path,omitempty" yaml:"path,omitempty"`
	PathType    string                     `json:"pathType,omitempty" yaml:"pathType,omitempty"`
	Resource    *TypedLocalObjectReference `json:"resource,omitempty" yaml:"resource,omitempty"`
	ServiceID   string                     `json:"serviceId,omitempty" yaml:"serviceId,omitempty"`
	TargetPort  intstr.IntOrString         `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
	WorkloadIDs []string                   `json:"workloadIds,omitempty" yaml:"workloadIds,omitempty"`
}
