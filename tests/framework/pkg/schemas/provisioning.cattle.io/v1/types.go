package schema

// Duration is a replacement struct for the Duration in the "k8s.io/apimachinery/pkg/apis/meta/v1" package
type Duration struct {
	Duration string `json:"duration,omitempty" yaml:"duration,omitempty"`
}

// MachineGlobalConfig is struct that defines the CNI for a RKEConfig need to provision a v1 cluster
type MachineGlobalConfig struct {
	CNI               string `json:"cni,omitempty" yaml:"cni,omitempty"`
	SecretsEncryption string `json:"secrets-encryption,omitempty" yaml:"secrets-encryption,omitempty"`
}
