package client

const (
	ProjectedVolumeSourceType             = "projectedVolumeSource"
	ProjectedVolumeSourceFieldDefaultMode = "defaultMode"
	ProjectedVolumeSourceFieldDefaultUser = "defaultUser"
	ProjectedVolumeSourceFieldSources     = "sources"
)

type ProjectedVolumeSource struct {
	DefaultMode *int64             `json:"defaultMode,omitempty" yaml:"defaultMode,omitempty"`
	DefaultUser *int64             `json:"defaultUser,omitempty" yaml:"defaultUser,omitempty"`
	Sources     []VolumeProjection `json:"sources,omitempty" yaml:"sources,omitempty"`
}
