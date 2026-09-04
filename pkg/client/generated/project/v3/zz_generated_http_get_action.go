package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	HTTPGetActionType             = "httpGetAction"
	HTTPGetActionFieldHTTPHeaders = "httpHeaders"
	HTTPGetActionFieldHost        = "host"
	HTTPGetActionFieldPath        = "path"
	HTTPGetActionFieldPort        = "port"
	HTTPGetActionFieldProtocol    = "protocol"
	HTTPGetActionFieldScheme      = "scheme"
)

type HTTPGetAction struct {
	HTTPHeaders []HTTPHeader       `json:"httpHeaders,omitempty" yaml:"httpHeaders,omitempty"`
	Host        string             `json:"host,omitempty" yaml:"host,omitempty"`
	Path        string             `json:"path,omitempty" yaml:"path,omitempty"`
	Port        intstr.IntOrString `json:"port,omitempty" yaml:"port,omitempty"`
	Protocol    string             `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Scheme      string             `json:"scheme,omitempty" yaml:"scheme,omitempty"`
}
