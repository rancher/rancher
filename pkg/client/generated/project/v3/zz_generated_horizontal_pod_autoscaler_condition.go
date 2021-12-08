package client

const (
	HorizontalPodAutoscalerConditionType                    = "horizontalPodAutoscalerCondition"
	HorizontalPodAutoscalerConditionFieldLastTransitionTime = "lastTransitionTime"
	HorizontalPodAutoscalerConditionFieldMessage            = "message"
	HorizontalPodAutoscalerConditionFieldReason             = "reason"
	HorizontalPodAutoscalerConditionFieldStatus             = "status"
	HorizontalPodAutoscalerConditionFieldType               = "type"
)

type HorizontalPodAutoscalerCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
