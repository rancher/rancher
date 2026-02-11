package client

const (
	MachineDeletionSpecType                                = "machineDeletionSpec"
	MachineDeletionSpecFieldNodeDeletionTimeoutSeconds     = "nodeDeletionTimeoutSeconds"
	MachineDeletionSpecFieldNodeDrainTimeoutSeconds        = "nodeDrainTimeoutSeconds"
	MachineDeletionSpecFieldNodeVolumeDetachTimeoutSeconds = "nodeVolumeDetachTimeoutSeconds"
)

type MachineDeletionSpec struct {
	NodeDeletionTimeoutSeconds     *int64 `json:"nodeDeletionTimeoutSeconds,omitempty" yaml:"nodeDeletionTimeoutSeconds,omitempty"`
	NodeDrainTimeoutSeconds        *int64 `json:"nodeDrainTimeoutSeconds,omitempty" yaml:"nodeDrainTimeoutSeconds,omitempty"`
	NodeVolumeDetachTimeoutSeconds *int64 `json:"nodeVolumeDetachTimeoutSeconds,omitempty" yaml:"nodeVolumeDetachTimeoutSeconds,omitempty"`
}
