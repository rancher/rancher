package client

const (
	ConditionType                    = "condition"
	ConditionFieldLastTransitionTime = "lastTransitionTime"
	ConditionFieldMessage            = "message"
	ConditionFieldObservedGeneration = "observedGeneration"
	ConditionFieldReason             = "reason"
	ConditionFieldStatus             = "status"
	ConditionFieldType               = "type"
)

type Condition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
