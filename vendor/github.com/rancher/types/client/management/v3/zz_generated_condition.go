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
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
