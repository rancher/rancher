package client

const (
	NodeTemplateConditionType                    = "nodeTemplateCondition"
	NodeTemplateConditionFieldLastTransitionTime = "lastTransitionTime"
	NodeTemplateConditionFieldLastUpdateTime     = "lastUpdateTime"
	NodeTemplateConditionFieldReason             = "reason"
	NodeTemplateConditionFieldStatus             = "status"
	NodeTemplateConditionFieldType               = "type"
)

type NodeTemplateCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
