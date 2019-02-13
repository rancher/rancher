package client

const (
	AlidnsProviderConfigType            = "alidnsProviderConfig"
	AlidnsProviderConfigFieldAccessKey  = "accessKey"
	AlidnsProviderConfigFieldRootDomain = "rootDomain"
	AlidnsProviderConfigFieldSecretKey  = "secretKey"
)

type AlidnsProviderConfig struct {
	AccessKey  string `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	RootDomain string `json:"rootDomain,omitempty" yaml:"rootDomain,omitempty"`
	SecretKey  string `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
}
