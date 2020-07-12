package client

const (
	SamlLoginOutputType                = "samlLoginOutput"
	SamlLoginOutputFieldIdpRedirectURL = "idpRedirectUrl"
)

type SamlLoginOutput struct {
	IdpRedirectURL string `json:"idpRedirectUrl,omitempty" yaml:"idpRedirectUrl,omitempty"`
}
