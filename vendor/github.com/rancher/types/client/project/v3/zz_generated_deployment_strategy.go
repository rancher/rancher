package client

const (
	DeploymentStrategyType               = "deploymentStrategy"
	DeploymentStrategyFieldRollingUpdate = "rollingUpdate"
	DeploymentStrategyFieldType          = "type"
)

type DeploymentStrategy struct {
	RollingUpdate *RollingUpdateDeployment `json:"rollingUpdate,omitempty"`
	Type          string                   `json:"type,omitempty"`
}
