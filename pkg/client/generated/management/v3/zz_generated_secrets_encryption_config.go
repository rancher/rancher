package client


	

	


import (
	
)

const (
    SecretsEncryptionConfigType = "secretsEncryptionConfig"
	SecretsEncryptionConfigFieldCustomConfig = "customConfig"
	SecretsEncryptionConfigFieldEnabled = "enabled"
)

type SecretsEncryptionConfig struct {
        CustomConfig *EncryptionConfiguration `json:"customConfig,omitempty" yaml:"customConfig,omitempty"`
        Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

