package client

const (
	MachineGlobalConfigType                   = "machineGlobalConfig"
	MachineGlobalConfigFieldCNI               = "cni"
	MachineGlobalConfigFieldSecretsEncryption = "secrets-encryption"
)

type MachineGlobalConfig struct {
	CNI               string `json:"cni,omitempty" yaml:"cni,omitempty"`
	SecretsEncryption string `json:"secrets-encryption,omitempty" yaml:"secrets-encryption,omitempty"`
}
