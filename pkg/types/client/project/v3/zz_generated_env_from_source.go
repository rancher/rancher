package client

const (
	EnvFromSourceType              = "envFromSource"
	EnvFromSourceFieldConfigMapRef = "configMapRef"
	EnvFromSourceFieldPrefix       = "prefix"
	EnvFromSourceFieldSecretRef    = "secretRef"
)

type EnvFromSource struct {
	ConfigMapRef *ConfigMapEnvSource `json:"configMapRef,omitempty" yaml:"configMapRef,omitempty"`
	Prefix       string              `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	SecretRef    *SecretEnvSource    `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
}
