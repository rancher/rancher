package client

const (
	CloudCredentialSpecType                    = "cloudCredentialSpec"
	CloudCredentialSpecFieldDescription        = "description"
	CloudCredentialSpecFieldDisplayName        = "displayName"
	CloudCredentialSpecFieldS3CredentialConfig = "s3credentialConfig"
)

type CloudCredentialSpec struct {
	Description        string              `json:"description,omitempty" yaml:"description,omitempty"`
	DisplayName        string              `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	S3CredentialConfig *S3CredentialConfig `json:"s3credentialConfig,omitempty" yaml:"s3credentialConfig,omitempty"`
}
