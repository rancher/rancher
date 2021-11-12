package client

const (
	AlidnsProviderConfigType                   = "alidnsProviderConfig"
	AlidnsProviderConfigFieldAccessKey         = "accessKey"
	AlidnsProviderConfigFieldAdditionalOptions = "additionalOptions"
	AlidnsProviderConfigFieldSecretKey         = "secretKey"
)

type AlidnsProviderConfig struct {
	AccessKey         string            `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	AdditionalOptions map[string]string `json:"additionalOptions,omitempty" yaml:"additionalOptions,omitempty"`
	SecretKey         string            `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
}
