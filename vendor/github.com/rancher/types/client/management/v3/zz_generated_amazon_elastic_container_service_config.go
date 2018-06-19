package client

const (
	AmazonElasticContainerServiceConfigType              = "amazonElasticContainerServiceConfig"
	AmazonElasticContainerServiceConfigFieldAccessKey    = "accessKey"
	AmazonElasticContainerServiceConfigFieldInstanceType = "instanceType"
	AmazonElasticContainerServiceConfigFieldMaximumNodes = "maximumNodes"
	AmazonElasticContainerServiceConfigFieldMinimumNodes = "minimumNodes"
	AmazonElasticContainerServiceConfigFieldRegion       = "region"
	AmazonElasticContainerServiceConfigFieldSecretKey    = "secretKey"
)

type AmazonElasticContainerServiceConfig struct {
	AccessKey    string `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	InstanceType string `json:"instanceType,omitempty" yaml:"instanceType,omitempty"`
	MaximumNodes int64  `json:"maximumNodes,omitempty" yaml:"maximumNodes,omitempty"`
	MinimumNodes int64  `json:"minimumNodes,omitempty" yaml:"minimumNodes,omitempty"`
	Region       string `json:"region,omitempty" yaml:"region,omitempty"`
	SecretKey    string `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
}
