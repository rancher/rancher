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
	FSType          string                `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	ReadOnly        bool                  `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SecretRef       *LocalObjectReference `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
	VolumeName      string                `json:"volumeName,omitempty" yaml:"volumeName,omitempty"`
	VolumeNamespace string                `json:"volumeNamespace,omitempty" yaml:"volumeNamespace,omitempty"`
}
