package etcdsnapshot

const (
	ConfigurationFileKey = "snapshotInput"
)

type Config struct {
	UpgradeKubernetesVersion     string `json:"upgradeKubernetesVersion" yaml:"upgradeKubernetesVersion"`
	SnapshotRestore              string `json:"snapshotRestore" yaml:"snapshotRestore"`
	ControlPlaneConcurrencyValue string `json:"controlPlaneConcurrencyValue" yaml:"controlPlaneConcurrencyValue"`
	ControlPlaneUnavailableValue string `json:"controlPlaneUnavailableValue" yaml:"controlPlaneUnavailableValue"`
	WorkerConcurrencyValue       string `json:"workerConcurrencyValue" yaml:"workerConcurrencyValue"`
	WorkerUnavailableValue       string `json:"workerUnavailableValue" yaml:"workerUnavailableValue"`
	RecurringRestores            int    `json:"recurringRestores" yaml:"recurringRestores"`
}
