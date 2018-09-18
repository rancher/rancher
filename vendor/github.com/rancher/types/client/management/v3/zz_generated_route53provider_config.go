package client

const (
	Route53ProviderConfigType            = "route53ProviderConfig"
	Route53ProviderConfigFieldAccessKey  = "accessKey"
	Route53ProviderConfigFieldRootDomain = "rootDomain"
	Route53ProviderConfigFieldSecretKey  = "secretKey"
)

type Route53ProviderConfig struct {
	AccessKey  string `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	RootDomain string `json:"rootDomain,omitempty" yaml:"rootDomain,omitempty"`
	SecretKey  string `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
}
