package client

const (
	ClusterUpgradeStrategyType                   = "clusterUpgradeStrategy"
	ClusterUpgradeStrategyFieldDrainServerNodes  = "drainServerNodes"
	ClusterUpgradeStrategyFieldDrainWorkerNodes  = "drainWorkerNodes"
	ClusterUpgradeStrategyFieldServerConcurrency = "serverConcurrency"
	ClusterUpgradeStrategyFieldWorkerConcurrency = "workerConcurrency"
)

type ClusterUpgradeStrategy struct {
	DrainServerNodes  bool  `json:"drainServerNodes,omitempty" yaml:"drainServerNodes,omitempty"`
	DrainWorkerNodes  bool  `json:"drainWorkerNodes,omitempty" yaml:"drainWorkerNodes,omitempty"`
	ServerConcurrency int64 `json:"serverConcurrency,omitempty" yaml:"serverConcurrency,omitempty"`
	WorkerConcurrency int64 `json:"workerConcurrency,omitempty" yaml:"workerConcurrency,omitempty"`
}
