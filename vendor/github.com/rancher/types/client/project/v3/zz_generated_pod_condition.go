package client

const (
	PodConditionType                    = "podCondition"
	PodConditionFieldLastProbeTime      = "lastProbeTime"
	PodConditionFieldLastTransitionTime = "lastTransitionTime"
	PodConditionFieldMessage            = "message"
	PodConditionFieldReason             = "reason"
	PodConditionFieldStatus             = "status"
	PodConditionFieldType               = "type"
)

type PodCondition struct {
	LastProbeTime      string `json:"lastProbeTime,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
