package client

const (
	HostPathVolumeSourceType      = "hostPathVolumeSource"
	HostPathVolumeSourceFieldKind = "kind"
	HostPathVolumeSourceFieldPath = "path"
)

type HostPathVolumeSource struct {
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}
