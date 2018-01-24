package client

const (
	DeploymentParallelConfigType                         = "deploymentParallelConfig"
	DeploymentParallelConfigFieldMinReadySeconds         = "minReadySeconds"
	DeploymentParallelConfigFieldProgressDeadlineSeconds = "processDeadlineSeconds"
	DeploymentParallelConfigFieldStartFirst              = "startFirst"
)

type DeploymentParallelConfig struct {
	MinReadySeconds         *int64 `json:"minReadySeconds,omitempty"`
	ProgressDeadlineSeconds *int64 `json:"processDeadlineSeconds,omitempty"`
	StartFirst              *bool  `json:"startFirst,omitempty"`
}
