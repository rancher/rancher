package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	TCPSocketActionType      = "tcpSocketAction"
	TCPSocketActionFieldHost = "host"
	TCPSocketActionFieldPort = "port"
)

type TCPSocketAction struct {
	Host string             `json:"host,omitempty"`
	Port intstr.IntOrString `json:"port,omitempty"`
}
