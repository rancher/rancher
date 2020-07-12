package client

const (
	IngressSpecType                  = "ingressSpec"
	IngressSpecFieldBackend          = "backend"
	IngressSpecFieldIngressClassName = "ingressClassName"
	IngressSpecFieldRules            = "rules"
	IngressSpecFieldTLS              = "tls"
)

type IngressSpec struct {
	Backend          *IngressBackend `json:"backend,omitempty" yaml:"backend,omitempty"`
	IngressClassName string          `json:"ingressClassName,omitempty" yaml:"ingressClassName,omitempty"`
	Rules            []IngressRule   `json:"rules,omitempty" yaml:"rules,omitempty"`
	TLS              []IngressTLS    `json:"tls,omitempty" yaml:"tls,omitempty"`
}
