package client

const (
	ReplicationControllerConditionType                    = "replicationControllerCondition"
	ReplicationControllerConditionFieldLastTransitionTime = "lastTransitionTime"
	ReplicationControllerConditionFieldMessage            = "message"
	ReplicationControllerConditionFieldReason             = "reason"
	ReplicationControllerConditionFieldStatus             = "status"
	ReplicationControllerConditionFieldType               = "type"
)

type ReplicationControllerCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
