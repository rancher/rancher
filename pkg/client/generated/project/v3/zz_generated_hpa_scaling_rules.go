package client

const (
	HPAScalingRulesType                            = "hpaScalingRules"
	HPAScalingRulesFieldPolicies                   = "policies"
	HPAScalingRulesFieldSelectPolicy               = "selectPolicy"
	HPAScalingRulesFieldStabilizationWindowSeconds = "stabilizationWindowSeconds"
)

type HPAScalingRules struct {
	Policies                   []HPAScalingPolicy `json:"policies,omitempty" yaml:"policies,omitempty"`
	SelectPolicy               string             `json:"selectPolicy,omitempty" yaml:"selectPolicy,omitempty"`
	StabilizationWindowSeconds *int64             `json:"stabilizationWindowSeconds,omitempty" yaml:"stabilizationWindowSeconds,omitempty"`
}
