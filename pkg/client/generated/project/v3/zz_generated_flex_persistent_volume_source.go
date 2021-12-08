package client

const (
	FlexPersistentVolumeSourceType           = "flexPersistentVolumeSource"
	FlexPersistentVolumeSourceFieldDriver    = "driver"
	FlexPersistentVolumeSourceFieldFSType    = "fsType"
	FlexPersistentVolumeSourceFieldOptions   = "options"
	FlexPersistentVolumeSourceFieldReadOnly  = "readOnly"
	FlexPersistentVolumeSourceFieldSecretRef = "secretRef"
)

type FlexPersistentVolumeSource struct {
	Driver    string            `json:"driver,omitempty" yaml:"driver,omitempty"`
	FSType    string            `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	Options   map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
	ReadOnly  bool              `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SecretRef *SecretReference  `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
}
