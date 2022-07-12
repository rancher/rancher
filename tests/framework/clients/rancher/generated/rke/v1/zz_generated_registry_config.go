package client

const (
	RegistryConfigType                      = "registryConfig"
	RegistryConfigFieldAuthConfigSecretName = "authConfigSecretName"
	RegistryConfigFieldCABundle             = "caBundle"
	RegistryConfigFieldInsecureSkipVerify   = "insecureSkipVerify"
	RegistryConfigFieldTLSSecretName        = "tlsSecretName"
)

type RegistryConfig struct {
	AuthConfigSecretName string `json:"authConfigSecretName,omitempty" yaml:"authConfigSecretName,omitempty"`
	CABundle             string `json:"caBundle,omitempty" yaml:"caBundle,omitempty"`
	InsecureSkipVerify   bool   `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
	TLSSecretName        string `json:"tlsSecretName,omitempty" yaml:"tlsSecretName,omitempty"`
}
