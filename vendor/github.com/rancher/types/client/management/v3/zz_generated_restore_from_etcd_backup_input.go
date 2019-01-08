package client

const (
	RestoreFromEtcdBackupInputType              = "restoreFromEtcdBackupInput"
	RestoreFromEtcdBackupInputFieldEtcdBackupID = "etcdBackupId"
)

type RestoreFromEtcdBackupInput struct {
	EtcdBackupID string `json:"etcdBackupId,omitempty" yaml:"etcdBackupId,omitempty"`
}
