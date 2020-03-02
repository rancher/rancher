package client

const (
	K3sConfigType                    = "k3sConfig"
	K3sConfigFieldK3sUpgradeStrategy = "k3supgradeStrategy"
	K3sConfigFieldVersion            = "kubernetesVersion"
)

type K3sConfig struct {
	K3sUpgradeStrategy *K3sUpgradeStrategy `json:"k3supgradeStrategy,omitempty" yaml:"k3supgradeStrategy,omitempty"`
	Version            string              `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
}
