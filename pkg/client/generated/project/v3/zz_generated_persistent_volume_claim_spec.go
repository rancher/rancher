package client

const (
	PersistentVolumeClaimSpecType                = "persistentVolumeClaimSpec"
	PersistentVolumeClaimSpecFieldAccessModes    = "accessModes"
	PersistentVolumeClaimSpecFieldDataSource     = "dataSource"
	PersistentVolumeClaimSpecFieldDataSourceRef  = "dataSourceRef"
	PersistentVolumeClaimSpecFieldResources      = "resources"
	PersistentVolumeClaimSpecFieldSelector       = "selector"
	PersistentVolumeClaimSpecFieldStorageClassID = "storageClassId"
	PersistentVolumeClaimSpecFieldVolumeID       = "volumeId"
	PersistentVolumeClaimSpecFieldVolumeMode     = "volumeMode"
)

type PersistentVolumeClaimSpec struct {
	AccessModes    []string                   `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
	DataSource     *TypedLocalObjectReference `json:"dataSource,omitempty" yaml:"dataSource,omitempty"`
	DataSourceRef  *TypedObjectReference      `json:"dataSourceRef,omitempty" yaml:"dataSourceRef,omitempty"`
	Resources      *ResourceRequirements      `json:"resources,omitempty" yaml:"resources,omitempty"`
	Selector       *LabelSelector             `json:"selector,omitempty" yaml:"selector,omitempty"`
	StorageClassID string                     `json:"storageClassId,omitempty" yaml:"storageClassId,omitempty"`
	VolumeID       string                     `json:"volumeId,omitempty" yaml:"volumeId,omitempty"`
	VolumeMode     string                     `json:"volumeMode,omitempty" yaml:"volumeMode,omitempty"`
}
