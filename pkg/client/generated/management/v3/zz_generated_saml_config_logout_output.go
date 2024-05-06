package client

const (
	SamlConfigLogoutOutputType                = "samlConfigLogoutOutput"
	SamlConfigLogoutOutputFieldIdpRedirectURL = "idpRedirectUrl"
)

type SamlConfigLogoutOutput struct {
	IdpRedirectURL string `json:"idpRedirectUrl,omitempty" yaml:"idpRedirectUrl,omitempty"`
}
