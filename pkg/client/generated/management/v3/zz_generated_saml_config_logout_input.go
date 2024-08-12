package client

const (
	SamlConfigLogoutInputType                  = "samlConfigLogoutInput"
	SamlConfigLogoutInputFieldFinalRedirectURL = "finalRedirectUrl"
)

type SamlConfigLogoutInput struct {
	FinalRedirectURL string `json:"finalRedirectUrl,omitempty" yaml:"finalRedirectUrl,omitempty"`
}
