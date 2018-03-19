package client

const (
	StorageOSPersistentVolumeSourceType                 = "storageOSPersistentVolumeSource"
	StorageOSPersistentVolumeSourceFieldFSType          = "fsType"
	StorageOSPersistentVolumeSourceFieldReadOnly        = "readOnly"
	StorageOSPersistentVolumeSourceFieldSecretRef       = "secretRef"
	StorageOSPersistentVolumeSourceFieldVolumeName      = "volumeName"
	StorageOSPersistentVolumeSourceFieldVolumeNamespace = "volumeNamespace"
)

type StorageOSPersistentVolumeSource struct {
	FSType          string           `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	ReadOnly        bool             `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SecretRef       *ObjectReference `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
	VolumeName      string           `json:"volumeName,omitempty" yaml:"volumeName,omitempty"`
	VolumeNamespace string           `json:"volumeNamespace,omitempty" yaml:"volumeNamespace,omitempty"`
}
