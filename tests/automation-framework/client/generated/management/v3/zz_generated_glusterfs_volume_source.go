package client

const (
	GlusterfsVolumeSourceType               = "glusterfsVolumeSource"
	GlusterfsVolumeSourceFieldEndpointsName = "endpoints"
	GlusterfsVolumeSourceFieldPath          = "path"
	GlusterfsVolumeSourceFieldReadOnly      = "readOnly"
)

type GlusterfsVolumeSource struct {
	EndpointsName string `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	Path          string `json:"path,omitempty" yaml:"path,omitempty"`
	ReadOnly      bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
}
