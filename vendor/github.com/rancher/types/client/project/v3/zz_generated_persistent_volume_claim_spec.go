package client

const (
	PersistentVolumeClaimSpecType                = "persistentVolumeClaimSpec"
	PersistentVolumeClaimSpecFieldAccessModes    = "accessModes"
	PersistentVolumeClaimSpecFieldResources      = "resources"
	PersistentVolumeClaimSpecFieldSelector       = "selector"
	PersistentVolumeClaimSpecFieldStorageClassId = "storageClassId"
	PersistentVolumeClaimSpecFieldVolumeId       = "volumeId"
)

type PersistentVolumeClaimSpec struct {
	AccessModes    []string              `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
	Resources      *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	Selector       *LabelSelector        `json:"selector,omitempty" yaml:"selector,omitempty"`
	StorageClassId string                `json:"storageClassId,omitempty" yaml:"storageClassId,omitempty"`
	VolumeId       string                `json:"volumeId,omitempty" yaml:"volumeId,omitempty"`
}
