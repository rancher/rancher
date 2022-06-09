package client

const (
	FlexVolumeSourceType           = "flexVolumeSource"
	FlexVolumeSourceFieldDriver    = "driver"
	FlexVolumeSourceFieldFSType    = "fsType"
	FlexVolumeSourceFieldOptions   = "options"
	FlexVolumeSourceFieldReadOnly  = "readOnly"
	FlexVolumeSourceFieldSecretRef = "secretRef"
)

type FlexVolumeSource struct {
	Driver    string                `json:"driver,omitempty" yaml:"driver,omitempty"`
	FSType    string                `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	Options   map[string]string     `json:"options,omitempty" yaml:"options,omitempty"`
	ReadOnly  bool                  `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SecretRef *LocalObjectReference `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
}
