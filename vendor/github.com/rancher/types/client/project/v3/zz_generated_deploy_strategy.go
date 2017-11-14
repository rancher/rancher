package client

const (
	DeployStrategyType                = "deployStrategy"
	DeployStrategyFieldGlobalConfig   = "globalConfig"
	DeployStrategyFieldJobConfig      = "jobConfig"
	DeployStrategyFieldKind           = "kind"
	DeployStrategyFieldOrderedConfig  = "orderedConfig"
	DeployStrategyFieldParallelConfig = "parallelConfig"
)

type DeployStrategy struct {
	GlobalConfig   *DeploymentGlobalConfig   `json:"globalConfig,omitempty"`
	JobConfig      *DeploymentJobConfig      `json:"jobConfig,omitempty"`
	Kind           string                    `json:"kind,omitempty"`
	OrderedConfig  *DeploymentOrderedConfig  `json:"orderedConfig,omitempty"`
	ParallelConfig *DeploymentParallelConfig `json:"parallelConfig,omitempty"`
}
