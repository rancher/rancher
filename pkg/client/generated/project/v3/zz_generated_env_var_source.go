package client

const (
	EnvVarSourceType                  = "envVarSource"
	EnvVarSourceFieldConfigMapKeyRef  = "configMapKeyRef"
	EnvVarSourceFieldFieldRef         = "fieldRef"
	EnvVarSourceFieldFileKeyRef       = "fileKeyRef"
	EnvVarSourceFieldResourceFieldRef = "resourceFieldRef"
	EnvVarSourceFieldSecretKeyRef     = "secretKeyRef"
)

type EnvVarSource struct {
	ConfigMapKeyRef  *ConfigMapKeySelector  `json:"configMapKeyRef,omitempty" yaml:"configMapKeyRef,omitempty"`
	FieldRef         *ObjectFieldSelector   `json:"fieldRef,omitempty" yaml:"fieldRef,omitempty"`
	FileKeyRef       *FileKeySelector       `json:"fileKeyRef,omitempty" yaml:"fileKeyRef,omitempty"`
	ResourceFieldRef *ResourceFieldSelector `json:"resourceFieldRef,omitempty" yaml:"resourceFieldRef,omitempty"`
	SecretKeyRef     *SecretKeySelector     `json:"secretKeyRef,omitempty" yaml:"secretKeyRef,omitempty"`
}
