package client

const (
	S3BackupConfigType            = "s3BackupConfig"
	S3BackupConfigFieldAccessKey  = "accessKey"
	S3BackupConfigFieldBucketName = "bucketName"
	S3BackupConfigFieldCustomCA   = "customCa"
	S3BackupConfigFieldEndpoint   = "endpoint"
	S3BackupConfigFieldFolder     = "folder"
	S3BackupConfigFieldRegion     = "region"
	S3BackupConfigFieldSecretKey  = "secretKey"
)

type S3BackupConfig struct {
	AccessKey  string `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	BucketName string `json:"bucketName,omitempty" yaml:"bucketName,omitempty"`
	CustomCA   string `json:"customCa,omitempty" yaml:"customCa,omitempty"`
	Endpoint   string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Folder     string `json:"folder,omitempty" yaml:"folder,omitempty"`
	Region     string `json:"region,omitempty" yaml:"region,omitempty"`
	SecretKey  string `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
}
