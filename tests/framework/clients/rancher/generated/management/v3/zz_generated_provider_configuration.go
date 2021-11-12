package client

const (
	ProviderConfigurationType           = "providerConfiguration"
	ProviderConfigurationFieldAESCBC    = "aescbc"
	ProviderConfigurationFieldAESGCM    = "aesgcm"
	ProviderConfigurationFieldIdentity  = "identity"
	ProviderConfigurationFieldKMS       = "kms"
	ProviderConfigurationFieldSecretbox = "secretbox"
)

type ProviderConfiguration struct {
	AESCBC    *AESConfiguration       `json:"aescbc,omitempty" yaml:"aescbc,omitempty"`
	AESGCM    *AESConfiguration       `json:"aesgcm,omitempty" yaml:"aesgcm,omitempty"`
	Identity  *IdentityConfiguration  `json:"identity,omitempty" yaml:"identity,omitempty"`
	KMS       *KMSConfiguration       `json:"kms,omitempty" yaml:"kms,omitempty"`
	Secretbox *SecretboxConfiguration `json:"secretbox,omitempty" yaml:"secretbox,omitempty"`
}
