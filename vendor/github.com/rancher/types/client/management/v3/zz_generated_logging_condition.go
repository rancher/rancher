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
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
