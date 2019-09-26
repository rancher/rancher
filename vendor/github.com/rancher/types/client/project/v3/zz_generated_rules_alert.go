package client

const (
	RulesAlertType                    = "rulesAlert"
	RulesAlertFieldForGracePeriod     = "forGracePeriod"
	RulesAlertFieldForOutageTolerance = "forOutageTolerance"
	RulesAlertFieldResendDelay        = "resendDelay"
)

type RulesAlert struct {
	ForGracePeriod     string `json:"forGracePeriod,omitempty" yaml:"forGracePeriod,omitempty"`
	ForOutageTolerance string `json:"forOutageTolerance,omitempty" yaml:"forOutageTolerance,omitempty"`
	ResendDelay        string `json:"resendDelay,omitempty" yaml:"resendDelay,omitempty"`
}
