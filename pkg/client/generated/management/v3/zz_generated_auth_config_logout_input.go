package client

const (
	AuthConfigLogoutInputType                  = "authConfigLogoutInput"
	AuthConfigLogoutInputFieldFinalRedirectURL = "finalRedirectUrl"
)

type AuthConfigLogoutInput struct {
	FinalRedirectURL string `json:"finalRedirectUrl,omitempty" yaml:"finalRedirectUrl,omitempty"`
}
