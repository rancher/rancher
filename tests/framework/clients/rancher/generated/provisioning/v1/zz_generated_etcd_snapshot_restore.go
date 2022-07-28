package client

const (
	ETCDSnapshotRestoreType                  = "etcdSnapshotRestore"
	ETCDSnapshotRestoreFieldGeneration       = "generation"
	ETCDSnapshotRestoreFieldName             = "name"
	ETCDSnapshotRestoreFieldRestoreRKEConfig = "restoreRKEConfig"
)

type ETCDSnapshotRestore struct {
	Generation       int64  `json:"generation,omitempty" yaml:"generation,omitempty"`
	Name             string `json:"name,omitempty" yaml:"name,omitempty"`
	RestoreRKEConfig string `json:"restoreRKEConfig,omitempty" yaml:"restoreRKEConfig,omitempty"`
}
