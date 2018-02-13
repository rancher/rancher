package client

const (
	NodeDriverConditionType                    = "nodeDriverCondition"
	NodeDriverConditionFieldLastTransitionTime = "lastTransitionTime"
	NodeDriverConditionFieldLastUpdateTime     = "lastUpdateTime"
	NodeDriverConditionFieldMessage            = "message"
	NodeDriverConditionFieldReason             = "reason"
	NodeDriverConditionFieldStatus             = "status"
	NodeDriverConditionFieldType               = "type"
)

type NodeDriverCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
