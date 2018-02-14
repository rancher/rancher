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
	AccessModes    []string              `json:"accessModes,omitempty"`
	Resources      *ResourceRequirements `json:"resources,omitempty"`
	Selector       *LabelSelector        `json:"selector,omitempty"`
	StorageClassId string                `json:"storageClassId,omitempty"`
	VolumeId       string                `json:"volumeId,omitempty"`
}
