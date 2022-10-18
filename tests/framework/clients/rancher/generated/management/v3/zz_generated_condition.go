package client

const (
	ConditionType                    = "condition"
	ConditionFieldLastTransitionTime = "lastTransitionTime"
	ConditionFieldLastUpdateTime     = "lastUpdateTime"
	ConditionFieldMessage            = "message"
	ConditionFieldReason             = "reason"
	ConditionFieldStatus             = "status"
	ConditionFieldType               = "type"
)

type Condition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
