package v3

//K3sConfig provides desired configuration for k3s clusters
type K3sConfig struct {
	// k3s Kubernetes version, unset the value indicates an unmanaged cluster
	Version            string `yaml:"kubernetes_version" json:"kubernetesVersion,omitempty"`
	K3sUpgradeStrategy `yaml:"k3s_upgrade_strategy,omitempty" json:"k3supgradeStrategy,omitempty"`
}

//K3sUpgradeStrategy provides configuration to the downstream system-upgrade-controller
type K3sUpgradeStrategy struct {
	// How many controlplane nodes should be upgrade at time, defaults to 1
	ServerConcurrency int `yaml:"server_concurrency" json:"serverConcurrency,omitempty" norman:"min=1"`
	// How many workers should be upgraded at a time
	WorkerConcurrency int `yaml:"worker_concurrency" json:"workerConcurrency,omitempty" norman:"min=1"`
	// Whether controlplane nodes should be drained
	DrainServerNodes bool `yaml:"drain_server_nodes" json:"drainServerNodes,omitempty"`
	// Whether worker nodes should be drained
	DrainWorkerNodes bool `yaml:"drain_worker_nodes" json:"drainWorkerNodes,omitempty"`
}
