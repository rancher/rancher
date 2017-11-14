package client

const (
	PersistentVolumeClaimSpecType                  = "persistentVolumeClaimSpec"
	PersistentVolumeClaimSpecFieldAccessModes      = "accessModes"
	PersistentVolumeClaimSpecFieldResources        = "resources"
	PersistentVolumeClaimSpecFieldSelector         = "selector"
	PersistentVolumeClaimSpecFieldStorageClassName = "storageClassName"
	PersistentVolumeClaimSpecFieldVolumeName       = "volumeName"
)

type PersistentVolumeClaimSpec struct {
	AccessModes      []string              `json:"accessModes,omitempty"`
	Resources        *ResourceRequirements `json:"resources,omitempty"`
	Selector         *LabelSelector        `json:"selector,omitempty"`
	StorageClassName string                `json:"storageClassName,omitempty"`
	VolumeName       string                `json:"volumeName,omitempty"`
}
