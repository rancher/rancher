package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	LifecycleHandlerType             = "lifecycleHandler"
	LifecycleHandlerFieldCommand     = "command"
	LifecycleHandlerFieldHTTPHeaders = "httpHeaders"
	LifecycleHandlerFieldHost        = "host"
	LifecycleHandlerFieldPath        = "path"
	LifecycleHandlerFieldPort        = "port"
	LifecycleHandlerFieldScheme      = "scheme"
	LifecycleHandlerFieldSleep       = "sleep"
	LifecycleHandlerFieldTCP         = "tcp"
)

type LifecycleHandler struct {
	Command     []string           `json:"command,omitempty" yaml:"command,omitempty"`
	HTTPHeaders []HTTPHeader       `json:"httpHeaders,omitempty" yaml:"httpHeaders,omitempty"`
	Host        string             `json:"host,omitempty" yaml:"host,omitempty"`
	Path        string             `json:"path,omitempty" yaml:"path,omitempty"`
	Port        intstr.IntOrString `json:"port,omitempty" yaml:"port,omitempty"`
	Scheme      string             `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	Sleep       *SleepAction       `json:"sleep,omitempty" yaml:"sleep,omitempty"`
	TCP         bool               `json:"tcp,omitempty" yaml:"tcp,omitempty"`
}
