package client

const (
	ECRCredentialPluginType                    = "ecrCredentialPlugin"
	ECRCredentialPluginFieldAwsAccessKeyID     = "awsAccessKeyId"
	ECRCredentialPluginFieldAwsSecretAccessKey = "awsSecretAccessKey"
	ECRCredentialPluginFieldAwsSessionToken    = "awsAccessToken"
)

type ECRCredentialPlugin struct {
	AwsAccessKeyID     string `json:"awsAccessKeyId,omitempty" yaml:"awsAccessKeyId,omitempty"`
	AwsSecretAccessKey string `json:"awsSecretAccessKey,omitempty" yaml:"awsSecretAccessKey,omitempty"`
	AwsSessionToken    string `json:"awsAccessToken,omitempty" yaml:"awsAccessToken,omitempty"`
}
