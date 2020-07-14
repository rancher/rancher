package client

const (
	IngressRuleType       = "ingressRule"
	IngressRuleFieldHost  = "host"
	IngressRuleFieldPaths = "paths"
)

type IngressRule struct {
	Host  string            `json:"host,omitempty" yaml:"host,omitempty"`
	Paths []HTTPIngressPath `json:"paths,omitempty" yaml:"paths,omitempty"`
}
