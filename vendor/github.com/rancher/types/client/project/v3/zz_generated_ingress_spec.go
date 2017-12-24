package client

const (
	IngressSpecType         = "ingressSpec"
	IngressSpecFieldBackend = "backend"
	IngressSpecFieldRules   = "rules"
	IngressSpecFieldTLS     = "tls"
)

type IngressSpec struct {
	Backend *IngressBackend `json:"backend,omitempty"`
	Rules   []IngressRule   `json:"rules,omitempty"`
	TLS     []IngressTLS    `json:"tls,omitempty"`
}
