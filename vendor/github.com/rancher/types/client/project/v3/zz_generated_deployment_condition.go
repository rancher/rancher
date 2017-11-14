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
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
