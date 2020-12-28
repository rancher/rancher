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
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
