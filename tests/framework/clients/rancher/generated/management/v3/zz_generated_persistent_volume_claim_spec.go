package client

const (
	PersistentVolumeClaimSpecType                  = "persistentVolumeClaimSpec"
	PersistentVolumeClaimSpecFieldAccessModes      = "accessModes"
	PersistentVolumeClaimSpecFieldDataSource       = "dataSource"
	PersistentVolumeClaimSpecFieldResources        = "resources"
	PersistentVolumeClaimSpecFieldSelector         = "selector"
	PersistentVolumeClaimSpecFieldStorageClassName = "storageClassName"
	PersistentVolumeClaimSpecFieldVolumeMode       = "volumeMode"
	PersistentVolumeClaimSpecFieldVolumeName       = "volumeName"
)

type PersistentVolumeClaimSpec struct {
	AccessModes      []string                   `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
	DataSource       *TypedLocalObjectReference `json:"dataSource,omitempty" yaml:"dataSource,omitempty"`
	Resources        *ResourceRequirements      `json:"resources,omitempty" yaml:"resources,omitempty"`
	Selector         *LabelSelector             `json:"selector,omitempty" yaml:"selector,omitempty"`
	StorageClassName string                     `json:"storageClassName,omitempty" yaml:"storageClassName,omitempty"`
	VolumeMode       string                     `json:"volumeMode,omitempty" yaml:"volumeMode,omitempty"`
	VolumeName       string                     `json:"volumeName,omitempty" yaml:"volumeName,omitempty"`
}
