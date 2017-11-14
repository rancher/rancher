package client

const (
	DeployConfigType                    = "deployConfig"
	DeployConfigFieldBatchSize          = "batchSize"
	DeployConfigFieldDeploymentStrategy = "deploymentStrategy"
	DeployConfigFieldScale              = "scale"
)

type DeployConfig struct {
	BatchSize          string          `json:"batchSize,omitempty"`
	DeploymentStrategy *DeployStrategy `json:"deploymentStrategy,omitempty"`
	Scale              *int64          `json:"scale,omitempty"`
}
