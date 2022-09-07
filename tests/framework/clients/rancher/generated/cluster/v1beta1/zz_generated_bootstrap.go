package client

const (
	BootstrapType                = "bootstrap"
	BootstrapFieldConfigRef      = "configRef"
	BootstrapFieldDataSecretName = "dataSecretName"
)

type Bootstrap struct {
	ConfigRef      *ObjectReference `json:"configRef,omitempty" yaml:"configRef,omitempty"`
	DataSecretName string           `json:"dataSecretName,omitempty" yaml:"dataSecretName,omitempty"`
}
