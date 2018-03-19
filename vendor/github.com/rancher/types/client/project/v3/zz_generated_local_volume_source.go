package client

const (
	LocalVolumeSourceType      = "localVolumeSource"
	LocalVolumeSourceFieldPath = "path"
)

type LocalVolumeSource struct {
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}
