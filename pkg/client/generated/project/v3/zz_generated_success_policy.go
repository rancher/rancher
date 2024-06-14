package client

const (
	SuccessPolicyType       = "successPolicy"
	SuccessPolicyFieldRules = "rules"
)

type SuccessPolicy struct {
	Rules []SuccessPolicyRule `json:"rules,omitempty" yaml:"rules,omitempty"`
}
