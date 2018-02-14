package client

const (
	GlusterfsVolumeSourceType               = "glusterfsVolumeSource"
	GlusterfsVolumeSourceFieldEndpointsName = "endpoints"
	GlusterfsVolumeSourceFieldPath          = "path"
	GlusterfsVolumeSourceFieldReadOnly      = "readOnly"
)

type GlusterfsVolumeSource struct {
	EndpointsName string `json:"endpoints,omitempty"`
	Path          string `json:"path,omitempty"`
	ReadOnly      bool   `json:"readOnly,omitempty"`
}
