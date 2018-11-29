package client

const (
	ThanosS3SpecType                   = "thanosS3Spec"
	ThanosS3SpecFieldAccessKey         = "accessKey"
	ThanosS3SpecFieldBucket            = "bucket"
	ThanosS3SpecFieldEncryptSSE        = "encryptsse"
	ThanosS3SpecFieldEndpoint          = "endpoint"
	ThanosS3SpecFieldInsecure          = "insecure"
	ThanosS3SpecFieldSecretKey         = "secretKey"
	ThanosS3SpecFieldSignatureVersion2 = "signatureVersion2"
)

type ThanosS3Spec struct {
	AccessKey         *SecretKeySelector `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	Bucket            string             `json:"bucket,omitempty" yaml:"bucket,omitempty"`
	EncryptSSE        *bool              `json:"encryptsse,omitempty" yaml:"encryptsse,omitempty"`
	Endpoint          string             `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Insecure          *bool              `json:"insecure,omitempty" yaml:"insecure,omitempty"`
	SecretKey         *SecretKeySelector `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
	SignatureVersion2 *bool              `json:"signatureVersion2,omitempty" yaml:"signatureVersion2,omitempty"`
}
