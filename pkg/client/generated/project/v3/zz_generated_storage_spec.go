package client

const (
	StorageSpecType                     = "storageSpec"
	StorageSpecFieldDisableMountSubPath = "disableMountSubPath"
	StorageSpecFieldEmptyDir            = "emptyDir"
	StorageSpecFieldEphemeral           = "ephemeral"
	StorageSpecFieldVolumeClaimTemplate = "volumeClaimTemplate"
)

type StorageSpec struct {
	DisableMountSubPath bool                           `json:"disableMountSubPath,omitempty" yaml:"disableMountSubPath,omitempty"`
	EmptyDir            *EmptyDirVolumeSource          `json:"emptyDir,omitempty" yaml:"emptyDir,omitempty"`
	Ephemeral           *EphemeralVolumeSource         `json:"ephemeral,omitempty" yaml:"ephemeral,omitempty"`
	VolumeClaimTemplate *EmbeddedPersistentVolumeClaim `json:"volumeClaimTemplate,omitempty" yaml:"volumeClaimTemplate,omitempty"`
}
