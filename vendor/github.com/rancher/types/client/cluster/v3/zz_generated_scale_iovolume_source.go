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
	FSType           string                `json:"fsType,omitempty"`
	Gateway          string                `json:"gateway,omitempty"`
	ProtectionDomain string                `json:"protectionDomain,omitempty"`
	ReadOnly         bool                  `json:"readOnly,omitempty"`
	SSLEnabled       bool                  `json:"sslEnabled,omitempty"`
	SecretRef        *LocalObjectReference `json:"secretRef,omitempty"`
	StorageMode      string                `json:"storageMode,omitempty"`
	StoragePool      string                `json:"storagePool,omitempty"`
	System           string                `json:"system,omitempty"`
	VolumeName       string                `json:"volumeName,omitempty"`
}
