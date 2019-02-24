package client

const (
	AlidnsProviderConfigType           = "alidnsProviderConfig"
	AlidnsProviderConfigFieldAccessKey = "accessKey"
	AlidnsProviderConfigFieldSecretKey = "secretKey"
)

type AlidnsProviderConfig struct {
	AccessKey string `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
}
