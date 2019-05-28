package client

const (
	RandomConditionType                    = "randomCondition"
	RandomConditionFieldLastTransitionTime = "lastTransitionTime"
	RandomConditionFieldLastUpdateTime     = "lastUpdateTime"
	RandomConditionFieldMessage            = "message"
	RandomConditionFieldReason             = "reason"
	RandomConditionFieldStatus             = "status"
	RandomConditionFieldType               = "type"
)

type RandomCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
