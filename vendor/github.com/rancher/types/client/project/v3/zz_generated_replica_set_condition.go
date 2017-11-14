package client

const (
	ReplicaSetConditionType                    = "replicaSetCondition"
	ReplicaSetConditionFieldLastTransitionTime = "lastTransitionTime"
	ReplicaSetConditionFieldMessage            = "message"
	ReplicaSetConditionFieldReason             = "reason"
	ReplicaSetConditionFieldStatus             = "status"
	ReplicaSetConditionFieldType               = "type"
)

type ReplicaSetCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
