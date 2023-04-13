package client

const (
	CatalogSecretsType                  = "catalogSecrets"
	CatalogSecretsFieldCredentialSecret = "credentialSecret"
)

type CatalogSecrets struct {
	CredentialSecret string `json:"credentialSecret,omitempty" yaml:"credentialSecret,omitempty"`
}
