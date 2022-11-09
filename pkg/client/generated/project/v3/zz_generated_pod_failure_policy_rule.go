package client

const (
	PodFailurePolicyRuleType                 = "podFailurePolicyRule"
	PodFailurePolicyRuleFieldAction          = "action"
	PodFailurePolicyRuleFieldOnExitCodes     = "onExitCodes"
	PodFailurePolicyRuleFieldOnPodConditions = "onPodConditions"
)

type PodFailurePolicyRule struct {
	Action          string                                   `json:"action,omitempty" yaml:"action,omitempty"`
	OnExitCodes     *PodFailurePolicyOnExitCodesRequirement  `json:"onExitCodes,omitempty" yaml:"onExitCodes,omitempty"`
	OnPodConditions []PodFailurePolicyOnPodConditionsPattern `json:"onPodConditions,omitempty" yaml:"onPodConditions,omitempty"`
}
