package client

const (
	AmazonElasticContainerServiceConfigType           = "amazonElasticContainerServiceConfig"
	AmazonElasticContainerServiceConfigFieldAccessKey = "accessKey"
	AmazonElasticContainerServiceConfigFieldSecretKey = "secretKey"
)

type AmazonElasticContainerServiceConfig struct {
	AccessKey string `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
}
