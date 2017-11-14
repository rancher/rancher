package client

const (
	DeploymentJobConfigType                       = "deploymentJobConfig"
	DeploymentJobConfigFieldActiveDeadlineSeconds = "activeDeadlineSeconds"
	DeploymentJobConfigFieldBatchLimit            = "batchLimit"
	DeploymentJobConfigFieldOnDelete              = "onDelete"
)

type DeploymentJobConfig struct {
	ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`
	BatchLimit            *int64 `json:"batchLimit,omitempty"`
	OnDelete              *bool  `json:"onDelete,omitempty"`
}
