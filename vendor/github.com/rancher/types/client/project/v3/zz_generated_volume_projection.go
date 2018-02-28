package client

const (
	VolumeProjectionType             = "volumeProjection"
	VolumeProjectionFieldConfigMap   = "configMap"
	VolumeProjectionFieldDownwardAPI = "downwardAPI"
	VolumeProjectionFieldSecret      = "secret"
)

type VolumeProjection struct {
	ConfigMap   *ConfigMapProjection   `json:"configMap,omitempty" yaml:"configMap,omitempty"`
	DownwardAPI *DownwardAPIProjection `json:"downwardAPI,omitempty" yaml:"downwardAPI,omitempty"`
	Secret      *SecretProjection      `json:"secret,omitempty" yaml:"secret,omitempty"`
}
