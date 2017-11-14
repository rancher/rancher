package client

const (
	DeploymentGlobalConfigType                 = "deploymentGlobalConfig"
	DeploymentGlobalConfigFieldMinReadySeconds = "minReadySeconds"
	DeploymentGlobalConfigFieldOnDelete        = "onDelete"
)

type DeploymentGlobalConfig struct {
	MinReadySeconds *int64 `json:"minReadySeconds,omitempty"`
	OnDelete        *bool  `json:"onDelete,omitempty"`
}
