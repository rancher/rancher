package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	HandlerType             = "handler"
	HandlerFieldCommand     = "command"
	HandlerFieldHTTPHeaders = "httpHeaders"
	HandlerFieldHost        = "host"
	HandlerFieldPath        = "path"
	HandlerFieldPort        = "port"
	HandlerFieldScheme      = "scheme"
	HandlerFieldTCP         = "tcp"
)

type Handler struct {
	Command     []string           `json:"command,omitempty" yaml:"command,omitempty"`
	HTTPHeaders []HTTPHeader       `json:"httpHeaders,omitempty" yaml:"httpHeaders,omitempty"`
	Host        string             `json:"host,omitempty" yaml:"host,omitempty"`
	Path        string             `json:"path,omitempty" yaml:"path,omitempty"`
	Port        intstr.IntOrString `json:"port,omitempty" yaml:"port,omitempty"`
	Scheme      string             `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	TCP         bool               `json:"tcp,omitempty" yaml:"tcp,omitempty"`
}
