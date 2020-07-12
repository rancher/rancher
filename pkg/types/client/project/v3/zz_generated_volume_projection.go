package client

const (
	VolumeProjectionType                     = "volumeProjection"
	VolumeProjectionFieldConfigMap           = "configMap"
	VolumeProjectionFieldDownwardAPI         = "downwardAPI"
	VolumeProjectionFieldSecret              = "secret"
	VolumeProjectionFieldServiceAccountToken = "serviceAccountToken"
)

type VolumeProjection struct {
	ConfigMap           *ConfigMapProjection           `json:"configMap,omitempty" yaml:"configMap,omitempty"`
	DownwardAPI         *DownwardAPIProjection         `json:"downwardAPI,omitempty" yaml:"downwardAPI,omitempty"`
	Secret              *SecretProjection              `json:"secret,omitempty" yaml:"secret,omitempty"`
	ServiceAccountToken *ServiceAccountTokenProjection `json:"serviceAccountToken,omitempty" yaml:"serviceAccountToken,omitempty"`
}
