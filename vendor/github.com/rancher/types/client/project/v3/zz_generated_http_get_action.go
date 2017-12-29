package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	HTTPGetActionType             = "httpGetAction"
	HTTPGetActionFieldHTTPHeaders = "httpHeaders"
	HTTPGetActionFieldPath        = "path"
	HTTPGetActionFieldPort        = "port"
	HTTPGetActionFieldScheme      = "scheme"
)

type HTTPGetAction struct {
	HTTPHeaders []HTTPHeader       `json:"httpHeaders,omitempty"`
	Path        string             `json:"path,omitempty"`
	Port        intstr.IntOrString `json:"port,omitempty"`
	Scheme      string             `json:"scheme,omitempty"`
}
