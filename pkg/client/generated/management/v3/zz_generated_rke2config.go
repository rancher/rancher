package client

const (
	Rke2ConfigType                        = "rke2Config"
	Rke2ConfigFieldClusterUpgradeStrategy = "rke2upgradeStrategy"
	Rke2ConfigFieldETCD                   = "etcd"
	Rke2ConfigFieldVersion                = "kubernetesVersion"
)

type Rke2Config struct {
	ClusterUpgradeStrategy *ClusterUpgradeStrategy `json:"rke2upgradeStrategy,omitempty" yaml:"rke2upgradeStrategy,omitempty"`
	ETCD                   *ETCD                   `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Version                string                  `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
}
