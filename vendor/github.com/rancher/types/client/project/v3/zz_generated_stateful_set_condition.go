package client

const (
	StatefulSetConditionType                    = "statefulSetCondition"
	StatefulSetConditionFieldLastTransitionTime = "lastTransitionTime"
	StatefulSetConditionFieldMessage            = "message"
	StatefulSetConditionFieldReason             = "reason"
	StatefulSetConditionFieldStatus             = "status"
	StatefulSetConditionFieldType               = "type"
)

type StatefulSetCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
