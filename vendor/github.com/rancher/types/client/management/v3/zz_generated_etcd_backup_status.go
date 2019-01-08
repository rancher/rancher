package client

const (
	EtcdBackupStatusType            = "etcdBackupStatus"
	EtcdBackupStatusFieldConditions = "conditions"
)

type EtcdBackupStatus struct {
	Conditions []EtcdBackupCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
