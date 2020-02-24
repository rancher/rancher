package client

const (
	EtcdBackupSpecType                   = "etcdBackupSpec"
	EtcdBackupSpecFieldBackupConfig      = "backupConfig"
	EtcdBackupSpecFieldClusterID         = "clusterId"
	EtcdBackupSpecFieldClusterObject     = "clusterObject"
	EtcdBackupSpecFieldFilename          = "filename"
	EtcdBackupSpecFieldKubernetesVersion = "kubernetesVersion"
	EtcdBackupSpecFieldManual            = "manual"
)

type EtcdBackupSpec struct {
	BackupConfig      *BackupConfig `json:"backupConfig,omitempty" yaml:"backupConfig,omitempty"`
	ClusterID         string        `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	ClusterObject     string        `json:"clusterObject,omitempty" yaml:"clusterObject,omitempty"`
	Filename          string        `json:"filename,omitempty" yaml:"filename,omitempty"`
	KubernetesVersion string        `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	Manual            bool          `json:"manual,omitempty" yaml:"manual,omitempty"`
}
