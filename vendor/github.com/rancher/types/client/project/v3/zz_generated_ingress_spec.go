package client

const (
	IngressSpecType         = "ingressSpec"
	IngressSpecFieldBackend = "backend"
	IngressSpecFieldRules   = "rules"
	IngressSpecFieldTLS     = "tls"
)

type IngressSpec struct {
	Backend *IngressBackend `json:"backend,omitempty" yaml:"backend,omitempty"`
	Rules   []IngressRule   `json:"rules,omitempty" yaml:"rules,omitempty"`
	TLS     []IngressTLS    `json:"tls,omitempty" yaml:"tls,omitempty"`
}
