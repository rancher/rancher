package client

const (
	EtcdBackupStatusType                   = "etcdBackupStatus"
	EtcdBackupStatusFieldClusterObject     = "clusterObject"
	EtcdBackupStatusFieldConditions        = "conditions"
	EtcdBackupStatusFieldKubernetesVersion = "kubernetesVersion"
)

type EtcdBackupStatus struct {
	ClusterObject     string                `json:"clusterObject,omitempty" yaml:"clusterObject,omitempty"`
	Conditions        []EtcdBackupCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	KubernetesVersion string                `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
}
