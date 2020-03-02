package client

const (
	K3sUpgradeStrategyType                   = "k3sUpgradeStrategy"
	K3sUpgradeStrategyFieldServerConcurrency = "serverConcurrency"
	K3sUpgradeStrategyFieldWorkerConcurrency = "workerConcurrency"
)

type K3sUpgradeStrategy struct {
	ServerConcurrency int64 `json:"serverConcurrency,omitempty" yaml:"serverConcurrency,omitempty"`
	WorkerConcurrency int64 `json:"workerConcurrency,omitempty" yaml:"workerConcurrency,omitempty"`
}
