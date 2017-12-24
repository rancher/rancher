package client

const (
	HTTPIngressRuleValueType       = "httpIngressRuleValue"
	HTTPIngressRuleValueFieldPaths = "paths"
)

type HTTPIngressRuleValue struct {
	Paths []HTTPIngressPath `json:"paths,omitempty"`
}
