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
	FSType          string           `json:"fsType,omitempty"`
	ReadOnly        bool             `json:"readOnly,omitempty"`
	SecretRef       *ObjectReference `json:"secretRef,omitempty"`
	VolumeName      string           `json:"volumeName,omitempty"`
	VolumeNamespace string           `json:"volumeNamespace,omitempty"`
}
