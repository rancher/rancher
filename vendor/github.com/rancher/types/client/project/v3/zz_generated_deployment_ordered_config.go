package client

const (
	DeploymentOrderedConfigType               = "deploymentOrderedConfig"
	DeploymentOrderedConfigFieldOnDelete      = "onDelete"
	DeploymentOrderedConfigFieldPartition     = "partition"
	DeploymentOrderedConfigFieldPartitionSize = "partitionSize"
)

type DeploymentOrderedConfig struct {
	OnDelete      *bool  `json:"onDelete,omitempty"`
	Partition     *int64 `json:"partition,omitempty"`
	PartitionSize *int64 `json:"partitionSize,omitempty"`
}
