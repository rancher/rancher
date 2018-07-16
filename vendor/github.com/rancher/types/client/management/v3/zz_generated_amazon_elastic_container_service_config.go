package client

const (
	AmazonElasticContainerServiceConfigType                = "amazonElasticContainerServiceConfig"
	AmazonElasticContainerServiceConfigFieldAccessKey      = "accessKey"
	AmazonElasticContainerServiceConfigFieldInstanceType   = "instanceType"
	AmazonElasticContainerServiceConfigFieldMaximumNodes   = "maximumNodes"
	AmazonElasticContainerServiceConfigFieldMinimumNodes   = "minimumNodes"
	AmazonElasticContainerServiceConfigFieldRegion         = "region"
	AmazonElasticContainerServiceConfigFieldSecretKey      = "secretKey"
	AmazonElasticContainerServiceConfigFieldSecurityGroups = "securityGroups"
	AmazonElasticContainerServiceConfigFieldServiceRole    = "serviceRole"
	AmazonElasticContainerServiceConfigFieldSubnets        = "subnets"
	AmazonElasticContainerServiceConfigFieldVirtualNetwork = "virtualNetwork"
)

type AmazonElasticContainerServiceConfig struct {
	AccessKey      string   `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	InstanceType   string   `json:"instanceType,omitempty" yaml:"instanceType,omitempty"`
	MaximumNodes   int64    `json:"maximumNodes,omitempty" yaml:"maximumNodes,omitempty"`
	MinimumNodes   int64    `json:"minimumNodes,omitempty" yaml:"minimumNodes,omitempty"`
	Region         string   `json:"region,omitempty" yaml:"region,omitempty"`
	SecretKey      string   `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
	SecurityGroups []string `json:"securityGroups,omitempty" yaml:"securityGroups,omitempty"`
	ServiceRole    string   `json:"serviceRole,omitempty" yaml:"serviceRole,omitempty"`
	Subnets        []string `json:"subnets,omitempty" yaml:"subnets,omitempty"`
	VirtualNetwork string   `json:"virtualNetwork,omitempty" yaml:"virtualNetwork,omitempty"`
}
