package client

const (
	BackupConfigType                = "backupConfig"
	BackupConfigFieldIntervalHours  = "intervalHours"
	BackupConfigFieldRetention      = "retention"
	BackupConfigFieldS3BackupConfig = "s3BackupConfig"
)

type BackupConfig struct {
	IntervalHours  int64           `json:"intervalHours,omitempty" yaml:"intervalHours,omitempty"`
	Retention      int64           `json:"retention,omitempty" yaml:"retention,omitempty"`
	S3BackupConfig *S3BackupConfig `json:"s3BackupConfig,omitempty" yaml:"s3BackupConfig,omitempty"`
}
