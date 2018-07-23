package client

const (
	SamlLoginInputType                  = "samlLoginInput"
	SamlLoginInputFieldFinalRedirectURL = "finalRedirectUrl"
)

type SamlLoginInput struct {
	FinalRedirectURL string `json:"finalRedirectUrl,omitempty" yaml:"finalRedirectUrl,omitempty"`
}
