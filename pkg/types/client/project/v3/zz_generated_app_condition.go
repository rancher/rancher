package client

const (
	AppConditionType                    = "appCondition"
	AppConditionFieldLastTransitionTime = "lastTransitionTime"
	AppConditionFieldLastUpdateTime     = "lastUpdateTime"
	AppConditionFieldMessage            = "message"
	AppConditionFieldReason             = "reason"
	AppConditionFieldStatus             = "status"
	AppConditionFieldType               = "type"
)

type AppCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
