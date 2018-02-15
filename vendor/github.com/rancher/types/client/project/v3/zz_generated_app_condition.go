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
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
