package client

const (
	K3sConfigType                        = "k3sConfig"
	K3sConfigFieldClusterUpgradeStrategy = "k3supgradeStrategy"
	K3sConfigFieldETCD                   = "etcd"
	K3sConfigFieldVersion                = "kubernetesVersion"
)

type K3sConfig struct {
	ClusterUpgradeStrategy *ClusterUpgradeStrategy `json:"k3supgradeStrategy,omitempty" yaml:"k3supgradeStrategy,omitempty"`
	ETCD                   *ETCD                   `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Version                string                  `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
}
