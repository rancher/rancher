package client

const (
	QuobyteVolumeSourceType          = "quobyteVolumeSource"
	QuobyteVolumeSourceFieldGroup    = "group"
	QuobyteVolumeSourceFieldReadOnly = "readOnly"
	QuobyteVolumeSourceFieldRegistry = "registry"
	QuobyteVolumeSourceFieldUser     = "user"
	QuobyteVolumeSourceFieldVolume   = "volume"
)

type QuobyteVolumeSource struct {
	Group    string `json:"group,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty"`
	Registry string `json:"registry,omitempty"`
	User     string `json:"user,omitempty"`
	Volume   string `json:"volume,omitempty"`
}
