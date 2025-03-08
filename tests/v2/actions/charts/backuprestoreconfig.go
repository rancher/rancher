package charts

const (
	BackupRestoreConfigurationFileKey = "backupRestoreInput"
)

type BackupRestoreConfig struct {
	BackupName                string `json:"backupName" yaml:"backupName"`
	S3BucketName              string `json:"s3BucketName" yaml:"s3BucketName"`
	S3FolderName              string `json:"s3FolderName" yaml:"s3FolderName"`
	S3Region                  string `json:"s3Region" yaml:"s3Region"`
	S3Endpoint                string `json:"s3Endpoint" yaml:"s3Endpoint"`
	VolumeName                string `json:"volumeName" yaml:"volumeName"`
	CredentialSecretNamespace string `json:"credentialSecretNamespace" yaml:"credentialSecretNamespace"`
	ResourceSetName           string `json:"resourceSetName" yaml:"resourceSetName"`
	Prune                     bool   `json:"prune" yaml:"prune"`
	AccessKey                 string `json:"accessKey" yaml:"accessKey"`
	SecretKey                 string `json:"secretKey" yaml:"secretKey"`
	ClusterNamespace          string `json:"clusterNamespace" yaml:"clusterNamespace"`
}
