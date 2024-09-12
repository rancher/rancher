package etcdsnapshot

const (
	ConfigurationFileKey = "snapshotInput"
)

type ReplaceRoles struct {
	Etcd         bool `json:"etcd" yaml:"etcd"`
	ControlPlane bool `json:"controlPlane" yaml:"controlPlane"`
	Worker       bool `json:"worker" yaml:"worker"`
}

type Config struct {
	UpgradeKubernetesVersion     string        `json:"upgradeKubernetesVersion" yaml:"upgradeKubernetesVersion"`
	SnapshotRestore              string        `json:"snapshotRestore" yaml:"snapshotRestore"`
	ControlPlaneConcurrencyValue string        `json:"controlPlaneConcurrencyValue" yaml:"controlPlaneConcurrencyValue"`
	ControlPlaneUnavailableValue string        `json:"controlPlaneUnavailableValue" yaml:"controlPlaneUnavailableValue"`
	WorkerConcurrencyValue       string        `json:"workerConcurrencyValue" yaml:"workerConcurrencyValue"`
	WorkerUnavailableValue       string        `json:"workerUnavailableValue" yaml:"workerUnavailableValue"`
	RecurringRestores            int           `json:"recurringRestores" yaml:"recurringRestores"`
	ReplaceRoles                 *ReplaceRoles `json:"replaceRoles" yaml:"replaceRoles"`
}
