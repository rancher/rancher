package client

const (
	IngressControllerType                          = "ingressController"
	IngressControllerFieldCustomDefaultBackend     = "customDefaultBackend"
	IngressControllerFieldHTTPLoadBalancingEnabled = "httpLoadBalancingEnabled"
	IngressControllerFieldIngressProvider          = "ingressProvider"
)

type IngressController struct {
	CustomDefaultBackend     bool   `json:"customDefaultBackend,omitempty" yaml:"customDefaultBackend,omitempty"`
	HTTPLoadBalancingEnabled bool   `json:"httpLoadBalancingEnabled,omitempty" yaml:"httpLoadBalancingEnabled,omitempty"`
	IngressProvider          string `json:"ingressProvider,omitempty" yaml:"ingressProvider,omitempty"`
}
