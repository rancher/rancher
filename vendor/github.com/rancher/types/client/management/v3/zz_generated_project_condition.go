package client

const (
	ProjectConditionType                    = "projectCondition"
	ProjectConditionFieldLastTransitionTime = "lastTransitionTime"
	ProjectConditionFieldLastUpdateTime     = "lastUpdateTime"
	ProjectConditionFieldMessage            = "message"
	ProjectConditionFieldReason             = "reason"
	ProjectConditionFieldStatus             = "status"
	ProjectConditionFieldType               = "type"
)

type ProjectCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
