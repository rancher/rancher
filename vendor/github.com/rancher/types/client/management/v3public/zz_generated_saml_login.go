package client

const (
	SamlLoginType                  = "samlLogin"
	SamlLoginFieldFinalRedirectURL = "finalRedirectUrl"
)

type SamlLogin struct {
	FinalRedirectURL string `json:"finalRedirectUrl,omitempty" yaml:"finalRedirectUrl,omitempty"`
}
