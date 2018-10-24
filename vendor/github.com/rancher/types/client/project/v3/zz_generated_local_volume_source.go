package client

const (
	LocalVolumeSourceType        = "localVolumeSource"
	LocalVolumeSourceFieldFSType = "fsType"
	LocalVolumeSourceFieldPath   = "path"
)

type LocalVolumeSource struct {
	FSType string `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	Path   string `json:"path,omitempty" yaml:"path,omitempty"`
}
