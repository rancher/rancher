package client

const (
	CSIVolumeSourceType                      = "csiVolumeSource"
	CSIVolumeSourceFieldDriver               = "driver"
	CSIVolumeSourceFieldFSType               = "fsType"
	CSIVolumeSourceFieldNodePublishSecretRef = "nodePublishSecretRef"
	CSIVolumeSourceFieldReadOnly             = "readOnly"
	CSIVolumeSourceFieldVolumeAttributes     = "volumeAttributes"
)

type CSIVolumeSource struct {
	Driver               string                `json:"driver,omitempty" yaml:"driver,omitempty"`
	FSType               string                `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	NodePublishSecretRef *LocalObjectReference `json:"nodePublishSecretRef,omitempty" yaml:"nodePublishSecretRef,omitempty"`
	ReadOnly             *bool                 `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	VolumeAttributes     map[string]string     `json:"volumeAttributes,omitempty" yaml:"volumeAttributes,omitempty"`
}
