package client

const (
	EnvVarSourceType                  = "envVarSource"
	EnvVarSourceFieldConfigMapKeyRef  = "configMapKeyRef"
	EnvVarSourceFieldFieldRef         = "fieldRef"
	EnvVarSourceFieldResourceFieldRef = "resourceFieldRef"
	EnvVarSourceFieldSecretKeyRef     = "secretKeyRef"
)

type EnvVarSource struct {
	ConfigMapKeyRef  *ConfigMapKeySelector  `json:"configMapKeyRef,omitempty"`
	FieldRef         *ObjectFieldSelector   `json:"fieldRef,omitempty"`
	ResourceFieldRef *ResourceFieldSelector `json:"resourceFieldRef,omitempty"`
	SecretKeyRef     *SecretKeySelector     `json:"secretKeyRef,omitempty"`
}
