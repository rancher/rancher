package client

const (
	StorageSpecType                     = "storageSpec"
	StorageSpecFieldDisableMountSubPath = "disableMountSubPath"
	StorageSpecFieldEmptyDir            = "emptyDir"
	StorageSpecFieldVolumeClaimTemplate = "volumeClaimTemplate"
)

type StorageSpec struct {
	DisableMountSubPath bool                           `json:"disableMountSubPath,omitempty" yaml:"disableMountSubPath,omitempty"`
	EmptyDir            *EmptyDirVolumeSource          `json:"emptyDir,omitempty" yaml:"emptyDir,omitempty"`
	VolumeClaimTemplate *EmbeddedPersistentVolumeClaim `json:"volumeClaimTemplate,omitempty" yaml:"volumeClaimTemplate,omitempty"`
}
