package client

const (
	AuthConfigLogoutOutputType                = "authConfigLogoutOutput"
	AuthConfigLogoutOutputFieldIdpRedirectURL = "idpRedirectUrl"
)

type AuthConfigLogoutOutput struct {
	IdpRedirectURL string `json:"idpRedirectUrl,omitempty" yaml:"idpRedirectUrl,omitempty"`
}
