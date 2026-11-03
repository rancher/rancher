package client

const (
	ConfigMapVolumeSourceType             = "configMapVolumeSource"
	ConfigMapVolumeSourceFieldDefaultMode = "defaultMode"
	ConfigMapVolumeSourceFieldDefaultUser = "defaultUser"
	ConfigMapVolumeSourceFieldItems       = "items"
	ConfigMapVolumeSourceFieldName        = "name"
	ConfigMapVolumeSourceFieldOptional    = "optional"
)

type ConfigMapVolumeSource struct {
	DefaultMode *int64      `json:"defaultMode,omitempty" yaml:"defaultMode,omitempty"`
	DefaultUser *int64      `json:"defaultUser,omitempty" yaml:"defaultUser,omitempty"`
	Items       []KeyToPath `json:"items,omitempty" yaml:"items,omitempty"`
	Name        string      `json:"name,omitempty" yaml:"name,omitempty"`
	Optional    *bool       `json:"optional,omitempty" yaml:"optional,omitempty"`
}
