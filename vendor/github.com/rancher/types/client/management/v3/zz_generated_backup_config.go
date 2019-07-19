package client

const (
	BackupConfigType                = "backupConfig"
	BackupConfigFieldEnabled        = "enabled"
	BackupConfigFieldIntervalHours  = "intervalHours"
	BackupConfigFieldRetention      = "retention"
	BackupConfigFieldS3BackupConfig = "s3BackupConfig"
)

type BackupConfig struct {
	Enabled        *bool           `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	IntervalHours  int64           `json:"intervalHours,omitempty" yaml:"intervalHours,omitempty"`
	Retention      int64           `json:"retention,omitempty" yaml:"retention,omitempty"`
	S3BackupConfig *S3BackupConfig `json:"s3BackupConfig,omitempty" yaml:"s3BackupConfig,omitempty"`
}
