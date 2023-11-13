package etcdsnapshot

const (
	ConfigurationFileKey = "snapshotInput"
)

type Config struct {
	UpgradeKubernetesVersion string `json:"upgradeKubernetesVersion" yaml:"upgradeKubernetesVersion"`
	SnapshotRestore          string `json:"snapshotRestore" yaml:"snapshotRestore"`
}
