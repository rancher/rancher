package client

const (
	NFSVolumeSourceType          = "nfsVolumeSource"
	NFSVolumeSourceFieldPath     = "path"
	NFSVolumeSourceFieldReadOnly = "readOnly"
	NFSVolumeSourceFieldServer   = "server"
)

type NFSVolumeSource struct {
	Path     string `json:"path,omitempty" yaml:"path,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	Server   string `json:"server,omitempty" yaml:"server,omitempty"`
}
