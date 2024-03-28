package client

const (
	PersistentVolumeClaimSpecType                           = "persistentVolumeClaimSpec"
	PersistentVolumeClaimSpecFieldAccessModes               = "accessModes"
	PersistentVolumeClaimSpecFieldDataSource                = "dataSource"
	PersistentVolumeClaimSpecFieldDataSourceRef             = "dataSourceRef"
	PersistentVolumeClaimSpecFieldResources                 = "resources"
	PersistentVolumeClaimSpecFieldSelector                  = "selector"
	PersistentVolumeClaimSpecFieldStorageClassName          = "storageClassName"
	PersistentVolumeClaimSpecFieldVolumeAttributesClassName = "volumeAttributesClassName"
	PersistentVolumeClaimSpecFieldVolumeMode                = "volumeMode"
	PersistentVolumeClaimSpecFieldVolumeName                = "volumeName"
)

type PersistentVolumeClaimSpec struct {
	AccessModes               []string                    `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
	DataSource                *TypedLocalObjectReference  `json:"dataSource,omitempty" yaml:"dataSource,omitempty"`
	DataSourceRef             *TypedObjectReference       `json:"dataSourceRef,omitempty" yaml:"dataSourceRef,omitempty"`
	Resources                 *VolumeResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	Selector                  *LabelSelector              `json:"selector,omitempty" yaml:"selector,omitempty"`
	StorageClassName          string                      `json:"storageClassName,omitempty" yaml:"storageClassName,omitempty"`
	VolumeAttributesClassName string                      `json:"volumeAttributesClassName,omitempty" yaml:"volumeAttributesClassName,omitempty"`
	VolumeMode                string                      `json:"volumeMode,omitempty" yaml:"volumeMode,omitempty"`
	VolumeName                string                      `json:"volumeName,omitempty" yaml:"volumeName,omitempty"`
}
