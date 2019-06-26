package client

const (
	APIServiceConditionType                    = "apiServiceCondition"
	APIServiceConditionFieldLastTransitionTime = "lastTransitionTime"
	APIServiceConditionFieldMessage            = "message"
	APIServiceConditionFieldReason             = "reason"
	APIServiceConditionFieldStatus             = "status"
	APIServiceConditionFieldType               = "type"
)

type APIServiceCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
