package client

const (
	IngressRuleType       = "ingressRule"
	IngressRuleFieldHost  = "host"
	IngressRuleFieldPaths = "paths"
)

type IngressRule struct {
	Host  string                     `json:"host,omitempty"`
	Paths map[string]HTTPIngressPath `json:"paths,omitempty"`
}
