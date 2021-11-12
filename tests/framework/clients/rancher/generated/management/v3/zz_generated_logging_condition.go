package client

const (
	LoggingConditionType                    = "loggingCondition"
	LoggingConditionFieldLastTransitionTime = "lastTransitionTime"
	LoggingConditionFieldLastUpdateTime     = "lastUpdateTime"
	LoggingConditionFieldMessage            = "message"
	LoggingConditionFieldReason             = "reason"
	LoggingConditionFieldStatus             = "status"
	LoggingConditionFieldType               = "type"
)

type LoggingCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
