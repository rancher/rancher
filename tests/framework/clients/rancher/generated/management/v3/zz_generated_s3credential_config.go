package client

const (
	S3CredentialConfigType                      = "s3CredentialConfig"
	S3CredentialConfigFieldAccessKey            = "accessKey"
	S3CredentialConfigFieldDefaultBucket        = "defaultBucket"
	S3CredentialConfigFieldDefaultEndpoint      = "defaultEndpoint"
	S3CredentialConfigFieldDefaultEndpointCA    = "defaultEndpointCA"
	S3CredentialConfigFieldDefaultFolder        = "defaultFolder"
	S3CredentialConfigFieldDefaultRegion        = "defaultRegion"
	S3CredentialConfigFieldDefaultSkipSSLVerify = "defaultSkipSSLVerify"
	S3CredentialConfigFieldSecretKey            = "secretKey"
)

type S3CredentialConfig struct {
	AccessKey            string `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	DefaultBucket        string `json:"defaultBucket,omitempty" yaml:"defaultBucket,omitempty"`
	DefaultEndpoint      string `json:"defaultEndpoint,omitempty" yaml:"defaultEndpoint,omitempty"`
	DefaultEndpointCA    string `json:"defaultEndpointCA,omitempty" yaml:"defaultEndpointCA,omitempty"`
	DefaultFolder        string `json:"defaultFolder,omitempty" yaml:"defaultFolder,omitempty"`
	DefaultRegion        string `json:"defaultRegion,omitempty" yaml:"defaultRegion,omitempty"`
	DefaultSkipSSLVerify string `json:"defaultSkipSSLVerify,omitempty" yaml:"defaultSkipSSLVerify,omitempty"`
	SecretKey            string `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
}
