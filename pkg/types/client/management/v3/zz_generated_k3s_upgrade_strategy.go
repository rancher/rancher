package client

const (
	K3sUpgradeStrategyType                   = "k3sUpgradeStrategy"
	K3sUpgradeStrategyFieldDrainServerNodes  = "drainServerNodes"
	K3sUpgradeStrategyFieldDrainWorkerNodes  = "drainWorkerNodes"
	K3sUpgradeStrategyFieldServerConcurrency = "serverConcurrency"
	K3sUpgradeStrategyFieldWorkerConcurrency = "workerConcurrency"
)

type K3sUpgradeStrategy struct {
	DrainServerNodes  bool  `json:"drainServerNodes,omitempty" yaml:"drainServerNodes,omitempty"`
	DrainWorkerNodes  bool  `json:"drainWorkerNodes,omitempty" yaml:"drainWorkerNodes,omitempty"`
	ServerConcurrency int64 `json:"serverConcurrency,omitempty" yaml:"serverConcurrency,omitempty"`
	WorkerConcurrency int64 `json:"workerConcurrency,omitempty" yaml:"workerConcurrency,omitempty"`
}
