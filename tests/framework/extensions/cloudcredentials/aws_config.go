package cloudcredentials

// The json/yaml config key for the aws cloud credential config
const AmazonEC2CredentialConfigurationFileKey = "awsCredentials"

// AmazonEC2CredentialConfig is configuration need to create an aws cloud credential
type AmazonEC2CredentialConfig struct {
	AccessKey     string `json:"accessKey" yaml:"accessKey"`
	SecretKey     string `json:"secretKey" yaml:"secretKey"`
	DefaultRegion string `json:"defaultRegion,omitempty" yaml:"defaultRegion"`
}
