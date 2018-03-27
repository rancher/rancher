package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	ServicePortType            = "servicePort"
	ServicePortFieldName       = "name"
	ServicePortFieldNodePort   = "nodePort"
	ServicePortFieldPort       = "port"
	ServicePortFieldProtocol   = "protocol"
	ServicePortFieldTargetPort = "targetPort"
)

type ServicePort struct {
	Name       string             `json:"name,omitempty" yaml:"name,omitempty"`
	NodePort   int64              `json:"nodePort,omitempty" yaml:"nodePort,omitempty"`
	Port       int64              `json:"port,omitempty" yaml:"port,omitempty"`
	Protocol   string             `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	TargetPort intstr.IntOrString `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
}
