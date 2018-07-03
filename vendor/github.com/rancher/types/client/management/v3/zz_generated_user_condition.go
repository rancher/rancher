package client

const (
	UserConditionType                    = "userCondition"
	UserConditionFieldLastTransitionTime = "lastTransitionTime"
	UserConditionFieldLastUpdateTime     = "lastUpdateTime"
	UserConditionFieldMessage            = "message"
	UserConditionFieldReason             = "reason"
	UserConditionFieldStatus             = "status"
	UserConditionFieldType               = "type"
)

type UserCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
