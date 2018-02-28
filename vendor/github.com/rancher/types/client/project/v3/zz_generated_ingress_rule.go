package client

const (
	IngressRuleType       = "ingressRule"
	IngressRuleFieldHost  = "host"
	IngressRuleFieldPaths = "paths"
)

type IngressRule struct {
	Host  string                    `json:"host,omitempty" yaml:"host,omitempty"`
	Paths map[string]IngressBackend `json:"paths,omitempty" yaml:"paths,omitempty"`
}
