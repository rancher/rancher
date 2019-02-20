package client

const (
	EtcdBackupSpecType              = "etcdBackupSpec"
	EtcdBackupSpecFieldBackupConfig = "backupConfig"
	EtcdBackupSpecFieldClusterID    = "clusterId"
	EtcdBackupSpecFieldFilename     = "filename"
)

type EtcdBackupSpec struct {
	BackupConfig *BackupConfig `json:"backupConfig,omitempty" yaml:"backupConfig,omitempty"`
	ClusterID    string        `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Filename     string        `json:"filename,omitempty" yaml:"filename,omitempty"`
}
