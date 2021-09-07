package client

const (
	GlusterfsPersistentVolumeSourceType                    = "glusterfsPersistentVolumeSource"
	GlusterfsPersistentVolumeSourceFieldEndpointsName      = "endpoints"
	GlusterfsPersistentVolumeSourceFieldEndpointsNamespace = "endpointsNamespace"
	GlusterfsPersistentVolumeSourceFieldPath               = "path"
	GlusterfsPersistentVolumeSourceFieldReadOnly           = "readOnly"
)

type GlusterfsPersistentVolumeSource struct {
	EndpointsName      string `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	EndpointsNamespace string `json:"endpointsNamespace,omitempty" yaml:"endpointsNamespace,omitempty"`
	Path               string `json:"path,omitempty" yaml:"path,omitempty"`
	ReadOnly           bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
}
