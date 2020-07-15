package client

const (
	RestoreConfigType              = "restoreConfig"
	RestoreConfigFieldRestore      = "restore"
	RestoreConfigFieldSnapshotName = "snapshotName"
)

type RestoreConfig struct {
	Restore      bool   `json:"restore,omitempty" yaml:"restore,omitempty"`
	SnapshotName string `json:"snapshotName,omitempty" yaml:"snapshotName,omitempty"`
}
