package client

const (
	NodeTemplateStatusType            = "nodeTemplateStatus"
	NodeTemplateStatusFieldConditions = "conditions"
)

type NodeTemplateStatus struct {
	Conditions []NodeTemplateCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
