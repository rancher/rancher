package client

const (
	BootstrapType                = "bootstrap"
	BootstrapFieldConfigRef      = "configRef"
	BootstrapFieldDataSecretName = "dataSecretName"
)

type Bootstrap struct {
	ConfigRef      *ContractVersionedObjectReference `json:"configRef,omitempty" yaml:"configRef,omitempty"`
	DataSecretName string                            `json:"dataSecretName,omitempty" yaml:"dataSecretName,omitempty"`
}
