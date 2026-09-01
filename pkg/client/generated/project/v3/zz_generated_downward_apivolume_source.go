package client

const (
	DownwardAPIVolumeSourceType             = "downwardAPIVolumeSource"
	DownwardAPIVolumeSourceFieldDefaultMode = "defaultMode"
	DownwardAPIVolumeSourceFieldDefaultUser = "defaultUser"
	DownwardAPIVolumeSourceFieldItems       = "items"
)

type DownwardAPIVolumeSource struct {
	DefaultMode *int64                  `json:"defaultMode,omitempty" yaml:"defaultMode,omitempty"`
	DefaultUser *int64                  `json:"defaultUser,omitempty" yaml:"defaultUser,omitempty"`
	Items       []DownwardAPIVolumeFile `json:"items,omitempty" yaml:"items,omitempty"`
}
