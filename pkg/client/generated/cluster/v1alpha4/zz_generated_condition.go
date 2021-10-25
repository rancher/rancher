package client

const (
	ConditionType                    = "condition"
	ConditionFieldLastTransitionTime = "lastTransitionTime"
	ConditionFieldMessage            = "message"
	ConditionFieldReason             = "reason"
	ConditionFieldSeverity           = "severity"
	ConditionFieldStatus             = "status"
	ConditionFieldType               = "type"
)

type Condition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Severity           string `json:"severity,omitempty" yaml:"severity,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
