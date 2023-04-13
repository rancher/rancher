package v3

// K3sConfig provides desired configuration for k3s clusters
type K3sConfig struct {
	Version                string `yaml:"kubernetes_version" json:"kubernetesVersion,omitempty"`
	ClusterUpgradeStrategy `yaml:"k3s_upgrade_strategy,omitempty" json:"k3supgradeStrategy,omitempty"`
}
type Rke2Config struct {
	Version                string `yaml:"kubernetes_version" json:"kubernetesVersion,omitempty"`
	ClusterUpgradeStrategy `yaml:"rke2_upgrade_strategy,omitempty" json:"rke2upgradeStrategy,omitempty"`
}

// ClusterUpgradeStrategy provides configuration to the downstream system-upgrade-controller
type ClusterUpgradeStrategy struct {
	// How many controlplane nodes should be upgrade at time, defaults to 1
	ServerConcurrency int `yaml:"server_concurrency" json:"serverConcurrency,omitempty" norman:"min=1"`
	// How many workers should be upgraded at a time
	WorkerConcurrency int `yaml:"worker_concurrency" json:"workerConcurrency,omitempty" norman:"min=1"`
	// Whether controlplane nodes should be drained
	DrainServerNodes bool `yaml:"drain_server_nodes" json:"drainServerNodes,omitempty"`
	// Whether worker nodes should be drained
	DrainWorkerNodes bool `yaml:"drain_worker_nodes" json:"drainWorkerNodes,omitempty"`
}

func (r *Rke2Config) SetStrategy(serverConcurrency, workerConcurrency int) {
	r.ClusterUpgradeStrategy.ServerConcurrency = serverConcurrency
	r.ClusterUpgradeStrategy.WorkerConcurrency = workerConcurrency
}
func (k *K3sConfig) SetStrategy(serverConcurrency, workerConcurrency int) {
	k.ClusterUpgradeStrategy.ServerConcurrency = serverConcurrency
	k.ClusterUpgradeStrategy.WorkerConcurrency = workerConcurrency
}
