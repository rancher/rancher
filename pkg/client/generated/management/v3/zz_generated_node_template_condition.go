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
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
