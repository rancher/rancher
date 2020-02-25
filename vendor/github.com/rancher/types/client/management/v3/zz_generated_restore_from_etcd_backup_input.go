package client

const (
	RestoreFromEtcdBackupInputType                  = "restoreFromEtcdBackupInput"
	RestoreFromEtcdBackupInputFieldEtcdBackupID     = "etcdBackupId"
	RestoreFromEtcdBackupInputFieldRestoreRkeConfig = "restoreRkeConfig"
)

type RestoreFromEtcdBackupInput struct {
	EtcdBackupID     string `json:"etcdBackupId,omitempty" yaml:"etcdBackupId,omitempty"`
	RestoreRkeConfig string `json:"restoreRkeConfig,omitempty" yaml:"restoreRkeConfig,omitempty"`
}
