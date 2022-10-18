package client

const (
	ClusterUpgradeStrategyType                          = "clusterUpgradeStrategy"
	ClusterUpgradeStrategyFieldControlPlaneConcurrency  = "controlPlaneConcurrency"
	ClusterUpgradeStrategyFieldControlPlaneDrainOptions = "controlPlaneDrainOptions"
	ClusterUpgradeStrategyFieldWorkerConcurrency        = "workerConcurrency"
	ClusterUpgradeStrategyFieldWorkerDrainOptions       = "workerDrainOptions"
)

type ClusterUpgradeStrategy struct {
	ControlPlaneConcurrency  string        `json:"controlPlaneConcurrency,omitempty" yaml:"controlPlaneConcurrency,omitempty"`
	ControlPlaneDrainOptions *DrainOptions `json:"controlPlaneDrainOptions,omitempty" yaml:"controlPlaneDrainOptions,omitempty"`
	WorkerConcurrency        string        `json:"workerConcurrency,omitempty" yaml:"workerConcurrency,omitempty"`
	WorkerDrainOptions       *DrainOptions `json:"workerDrainOptions,omitempty" yaml:"workerDrainOptions,omitempty"`
}
