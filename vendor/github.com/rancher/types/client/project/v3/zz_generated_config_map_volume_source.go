package client

const (
	ConfigMapVolumeSourceType             = "configMapVolumeSource"
	ConfigMapVolumeSourceFieldDefaultMode = "defaultMode"
	ConfigMapVolumeSourceFieldItems       = "items"
	ConfigMapVolumeSourceFieldName        = "name"
	ConfigMapVolumeSourceFieldOptional    = "optional"
)

type ConfigMapVolumeSource struct {
	DefaultMode *int64      `json:"defaultMode,omitempty"`
	Items       []KeyToPath `json:"items,omitempty"`
	Name        string      `json:"name,omitempty"`
	Optional    *bool       `json:"optional,omitempty"`
}
