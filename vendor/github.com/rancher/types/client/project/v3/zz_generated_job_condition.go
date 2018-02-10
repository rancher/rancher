package client

const (
	JobConditionType                    = "jobCondition"
	JobConditionFieldLastProbeTime      = "lastProbeTime"
	JobConditionFieldLastTransitionTime = "lastTransitionTime"
	JobConditionFieldMessage            = "message"
	JobConditionFieldReason             = "reason"
	JobConditionFieldStatus             = "status"
	JobConditionFieldType               = "type"
)

type JobCondition struct {
	LastProbeTime      string `json:"lastProbeTime,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
