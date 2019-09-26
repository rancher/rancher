package client

const (
	NamespaceConditionType                    = "namespaceCondition"
	NamespaceConditionFieldLastTransitionTime = "lastTransitionTime"
	NamespaceConditionFieldMessage            = "message"
	NamespaceConditionFieldReason             = "reason"
	NamespaceConditionFieldStatus             = "status"
	NamespaceConditionFieldType               = "type"
)

type NamespaceCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
