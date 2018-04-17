package client

const (
	SamlConfigTestInputType                  = "samlConfigTestInput"
	SamlConfigTestInputFieldFinalRedirectURL = "finalRedirectUrl"
)

type SamlConfigTestInput struct {
	FinalRedirectURL string `json:"finalRedirectUrl,omitempty" yaml:"finalRedirectUrl,omitempty"`
}
