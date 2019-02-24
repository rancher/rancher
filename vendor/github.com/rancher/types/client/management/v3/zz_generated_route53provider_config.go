package client

const (
	Route53ProviderConfigType           = "route53ProviderConfig"
	Route53ProviderConfigFieldAccessKey = "accessKey"
	Route53ProviderConfigFieldSecretKey = "secretKey"
)

type Route53ProviderConfig struct {
	AccessKey string `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
}
