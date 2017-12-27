package client

const (
	ClusterConditionType                    = "clusterCondition"
	ClusterConditionFieldLastTransitionTime = "lastTransitionTime"
	ClusterConditionFieldLastUpdateTime     = "lastUpdateTime"
	ClusterConditionFieldMessage            = "message"
	ClusterConditionFieldReason             = "reason"
	ClusterConditionFieldStatus             = "status"
	ClusterConditionFieldType               = "type"
)

type ClusterCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
