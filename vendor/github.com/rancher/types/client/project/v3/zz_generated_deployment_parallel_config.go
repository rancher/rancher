package client

const (
	DeploymentParallelConfigType                         = "deploymentParallelConfig"
	DeploymentParallelConfigFieldMinReadySeconds         = "minReadySeconds"
	DeploymentParallelConfigFieldProgressDeadlineSeconds = "progressDeadlineSeconds"
	DeploymentParallelConfigFieldStartFirst              = "startFirst"
)

type DeploymentParallelConfig struct {
	MinReadySeconds         *int64 `json:"minReadySeconds,omitempty"`
	ProgressDeadlineSeconds *int64 `json:"progressDeadlineSeconds,omitempty"`
	StartFirst              *bool  `json:"startFirst,omitempty"`
}
