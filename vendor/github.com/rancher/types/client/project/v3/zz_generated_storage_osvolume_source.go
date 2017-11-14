package client

const (
	StorageOSVolumeSourceType                 = "storageOSVolumeSource"
	StorageOSVolumeSourceFieldFSType          = "fsType"
	StorageOSVolumeSourceFieldReadOnly        = "readOnly"
	StorageOSVolumeSourceFieldSecretRef       = "secretRef"
	StorageOSVolumeSourceFieldVolumeName      = "volumeName"
	StorageOSVolumeSourceFieldVolumeNamespace = "volumeNamespace"
)

type StorageOSVolumeSource struct {
	FSType          string                `json:"fsType,omitempty"`
	ReadOnly        *bool                 `json:"readOnly,omitempty"`
	SecretRef       *LocalObjectReference `json:"secretRef,omitempty"`
	VolumeName      string                `json:"volumeName,omitempty"`
	VolumeNamespace string                `json:"volumeNamespace,omitempty"`
}
