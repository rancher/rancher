package client

const (
	StorageSpecType                     = "storageSpec"
	StorageSpecFieldEmptyDir            = "emptyDir"
	StorageSpecFieldVolumeClaimTemplate = "volumeClaimTemplate"
)

type StorageSpec struct {
	EmptyDir            *EmptyDirVolumeSource  `json:"emptyDir,omitempty" yaml:"emptyDir,omitempty"`
	VolumeClaimTemplate *PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty" yaml:"volumeClaimTemplate,omitempty"`
}
