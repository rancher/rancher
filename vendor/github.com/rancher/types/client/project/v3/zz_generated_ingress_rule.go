package client

const (
	IngressRuleType      = "ingressRule"
	IngressRuleFieldHTTP = "http"
	IngressRuleFieldHost = "host"
)

type IngressRule struct {
	HTTP *HTTPIngressRuleValue `json:"http,omitempty"`
	Host string                `json:"host,omitempty"`
}
