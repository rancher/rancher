package client

const (
	ScaleIOVolumeSourceType                  = "scaleIOVolumeSource"
	ScaleIOVolumeSourceFieldFSType           = "fsType"
	ScaleIOVolumeSourceFieldGateway          = "gateway"
	ScaleIOVolumeSourceFieldProtectionDomain = "protectionDomain"
	ScaleIOVolumeSourceFieldReadOnly         = "readOnly"
	ScaleIOVolumeSourceFieldSSLEnabled       = "sslEnabled"
	ScaleIOVolumeSourceFieldSecretRef        = "secretRef"
	ScaleIOVolumeSourceFieldStorageMode      = "storageMode"
	ScaleIOVolumeSourceFieldStoragePool      = "storagePool"
	ScaleIOVolumeSourceFieldSystem           = "system"
	ScaleIOVolumeSourceFieldVolumeName       = "volumeName"
)

type ScaleIOVolumeSource struct {
	FSType           string                `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	Gateway          string                `json:"gateway,omitempty" yaml:"gateway,omitempty"`
	ProtectionDomain string                `json:"protectionDomain,omitempty" yaml:"protectionDomain,omitempty"`
	ReadOnly         bool                  `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SSLEnabled       bool                  `json:"sslEnabled,omitempty" yaml:"sslEnabled,omitempty"`
	SecretRef        *LocalObjectReference `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
	StorageMode      string                `json:"storageMode,omitempty" yaml:"storageMode,omitempty"`
	StoragePool      string                `json:"storagePool,omitempty" yaml:"storagePool,omitempty"`
	System           string                `json:"system,omitempty" yaml:"system,omitempty"`
	VolumeName       string                `json:"volumeName,omitempty" yaml:"volumeName,omitempty"`
}
