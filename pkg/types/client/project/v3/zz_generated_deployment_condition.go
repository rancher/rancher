package client

const (
	DeploymentConditionType                    = "deploymentCondition"
	DeploymentConditionFieldLastTransitionTime = "lastTransitionTime"
	DeploymentConditionFieldLastUpdateTime     = "lastUpdateTime"
	DeploymentConditionFieldMessage            = "message"
	DeploymentConditionFieldReason             = "reason"
	DeploymentConditionFieldStatus             = "status"
	DeploymentConditionFieldType               = "type"
)

type DeploymentCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
