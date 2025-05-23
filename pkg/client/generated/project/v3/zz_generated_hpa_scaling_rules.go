package client

const (
	HPAScalingRulesType                            = "hpaScalingRules"
	HPAScalingRulesFieldPolicies                   = "policies"
	HPAScalingRulesFieldSelectPolicy               = "selectPolicy"
	HPAScalingRulesFieldStabilizationWindowSeconds = "stabilizationWindowSeconds"
	HPAScalingRulesFieldTolerance                  = "tolerance"
)

type HPAScalingRules struct {
	Policies                   []HPAScalingPolicy `json:"policies,omitempty" yaml:"policies,omitempty"`
	SelectPolicy               string             `json:"selectPolicy,omitempty" yaml:"selectPolicy,omitempty"`
	StabilizationWindowSeconds *int64             `json:"stabilizationWindowSeconds,omitempty" yaml:"stabilizationWindowSeconds,omitempty"`
	Tolerance                  string             `json:"tolerance,omitempty" yaml:"tolerance,omitempty"`
}
