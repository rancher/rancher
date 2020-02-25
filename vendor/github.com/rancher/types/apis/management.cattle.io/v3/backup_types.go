package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	BackupConditionCreated   condition.Cond = "Created"
	BackupConditionCompleted condition.Cond = "Completed"
)

type BackupConfig struct {
	// Enable or disable recurring backups in rancher
	Enabled *bool `yaml:"enabled" json:"enabled,omitempty" norman:"default=true"`
	// Backup interval in hours
	IntervalHours int `yaml:"interval_hours" json:"intervalHours,omitempty" norman:"default=12"`
	// Number of backups to keep
	Retention int `yaml:"retention" json:"retention,omitempty" norman:"default=6"`
	// s3 target
	S3BackupConfig *S3BackupConfig `yaml:",omitempty" json:"s3BackupConfig"`
	// replace special characters in snapshot names
	SafeTimestamp bool `yaml:"safe_timestamp" json:"safeTimestamp,omitempty"`
}

type S3BackupConfig struct {
	// Access key ID
	AccessKey string `yaml:"access_key" json:"accessKey,omitempty"`
	// Secret access key
	SecretKey string `yaml:"secret_key" json:"secretKey,omitempty" norman:"type=password" `
	// name of the bucket to use for backup
	BucketName string `yaml:"bucket_name" json:"bucketName,omitempty"`
	// AWS Region, AWS spcific
	Region string `yaml:"region" json:"region,omitempty"`
	// Endpoint is used if this is not an AWS API
	Endpoint string `yaml:"endpoint" json:"endpoint"`
	// CustomCA is used to connect to custom s3 endpoints
	CustomCA string `yaml:"custom_ca" json:"customCa,omitempty"`
	// Folder to place the files
	Folder string `yaml:"folder" json:"folder,omitempty"`
}
type EtcdBackup struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// backup spec
	Spec EtcdBackupSpec `json:"spec"`
	// backup status
	Status EtcdBackupStatus `yaml:"status" json:"status,omitempty"`
}

type EtcdBackupSpec struct {
	// cluster ID
	ClusterID string `json:"clusterId,omitempty" norman:"required,type=reference[cluster],noupdate"`
	// manual backup flag
	Manual bool `yaml:"manual" json:"manual,omitempty"`
	// actual file name on the target
	Filename string `yaml:"filename" json:"filename,omitempty" norman:"noupdate"`
	// backupConfig
	BackupConfig BackupConfig `yaml:",omitempty" json:"backupConfig,omitempty" norman:"noupdate"`
}

type EtcdBackupStatus struct {
	Conditions []EtcdBackupCondition `json:"conditions"`
	// version of k8s in the backup pulled from rke config
	KubernetesVersion string `yaml:"kubernetesVersion" json:"kubernetesVersion,omitempty" norman:"noupdate"`
	// json + gzipped + base64 backup of the cluster object when the backup was created
	ClusterObject string `yaml:"clusterObject" json:"clusterObject,omitempty" norman:"type=password,noupdate"`
}

type EtcdBackupCondition struct {
	// Type of condition.
	Type string `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}
