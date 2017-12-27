package client

const (
	HTTPIngressRuleValueType       = "httpIngressRuleValue"
	HTTPIngressRuleValueFieldPaths = "paths"
)

type HTTPIngressRuleValue struct {
	Paths map[string]HTTPIngressPath `json:"paths,omitempty"`
}
