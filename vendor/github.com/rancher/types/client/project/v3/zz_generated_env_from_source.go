package client

const (
	EnvFromSourceType              = "envFromSource"
	EnvFromSourceFieldConfigMapRef = "configMapRef"
	EnvFromSourceFieldPrefix       = "prefix"
	EnvFromSourceFieldSecretRef    = "secretRef"
)

type EnvFromSource struct {
	ConfigMapRef *ConfigMapEnvSource `json:"configMapRef,omitempty"`
	Prefix       string              `json:"prefix,omitempty"`
	SecretRef    *SecretEnvSource    `json:"secretRef,omitempty"`
}
