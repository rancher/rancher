package client

const (
	DeploymentStrategyType               = "deploymentStrategy"
	DeploymentStrategyFieldRollingUpdate = "rollingUpdate"
	DeploymentStrategyFieldStrategy      = "strategy"
)

type DeploymentStrategy struct {
	RollingUpdate *RollingUpdateDeployment `json:"rollingUpdate,omitempty" yaml:"rollingUpdate,omitempty"`
	Strategy      string                   `json:"strategy,omitempty" yaml:"strategy,omitempty"`
}
