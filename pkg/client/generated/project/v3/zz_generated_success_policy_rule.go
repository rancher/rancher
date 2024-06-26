package client

const (
	SuccessPolicyRuleType                  = "successPolicyRule"
	SuccessPolicyRuleFieldSucceededCount   = "succeededCount"
	SuccessPolicyRuleFieldSucceededIndexes = "succeededIndexes"
)

type SuccessPolicyRule struct {
	SucceededCount   *int64 `json:"succeededCount,omitempty" yaml:"succeededCount,omitempty"`
	SucceededIndexes string `json:"succeededIndexes,omitempty" yaml:"succeededIndexes,omitempty"`
}
