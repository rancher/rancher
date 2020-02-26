package client

const (
	SecretOrConfigMapType           = "secretOrConfigMap"
	SecretOrConfigMapFieldConfigMap = "configMap"
	SecretOrConfigMapFieldSecret    = "secret"
)

type SecretOrConfigMap struct {
	ConfigMap *ConfigMapKeySelector `json:"configMap,omitempty" yaml:"configMap,omitempty"`
	Secret    *SecretKeySelector    `json:"secret,omitempty" yaml:"secret,omitempty"`
}
