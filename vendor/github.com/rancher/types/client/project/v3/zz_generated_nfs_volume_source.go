package client

const (
	NFSVolumeSourceType          = "nfsVolumeSource"
	NFSVolumeSourceFieldPath     = "path"
	NFSVolumeSourceFieldReadOnly = "readOnly"
	NFSVolumeSourceFieldServer   = "server"
)

type NFSVolumeSource struct {
	Path     string `json:"path,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty"`
	Server   string `json:"server,omitempty"`
}
