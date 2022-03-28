package client

const (
	ClusterTemplateRevisionConditionType                    = "clusterTemplateRevisionCondition"
	ClusterTemplateRevisionConditionFieldLastTransitionTime = "lastTransitionTime"
	ClusterTemplateRevisionConditionFieldLastUpdateTime     = "lastUpdateTime"
	ClusterTemplateRevisionConditionFieldMessage            = "message"
	ClusterTemplateRevisionConditionFieldReason             = "reason"
	ClusterTemplateRevisionConditionFieldStatus             = "status"
	ClusterTemplateRevisionConditionFieldType               = "type"
)

type ClusterTemplateRevisionCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
