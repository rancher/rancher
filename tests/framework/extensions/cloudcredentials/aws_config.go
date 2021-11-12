package cloudcredentials

const AmazonEC2CredentialConfigurationFileKey = "awsCredentials"

type AmazonEC2CredentialConfig struct {
	AccessKey     string `json:"accessKey" yaml:"accessKey"`
	SecretKey     string `json:"secretKey" yaml:"secretKey"`
	DefaultRegion string `json:"defaultRegion,omitempty" yaml:"defaultRegion"`
}
