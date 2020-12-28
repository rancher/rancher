package client

const (
	DaemonSetConditionType                    = "daemonSetCondition"
	DaemonSetConditionFieldLastTransitionTime = "lastTransitionTime"
	DaemonSetConditionFieldMessage            = "message"
	DaemonSetConditionFieldReason             = "reason"
	DaemonSetConditionFieldStatus             = "status"
	DaemonSetConditionFieldType               = "type"
)

type DaemonSetCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
