package client

const (
	SamlLoginInputType                  = "samlLoginInput"
	SamlLoginInputFieldFinalRedirectURL = "finalRedirectUrl"
	SamlLoginInputFieldPublicKey        = "publicKey"
	SamlLoginInputFieldRequestID        = "requestId"
	SamlLoginInputFieldResponseType     = "responseType"
)

type SamlLoginInput struct {
	FinalRedirectURL string `json:"finalRedirectUrl,omitempty" yaml:"finalRedirectUrl,omitempty"`
	PublicKey        string `json:"publicKey,omitempty" yaml:"publicKey,omitempty"`
	RequestID        string `json:"requestId,omitempty" yaml:"requestId,omitempty"`
	ResponseType     string `json:"responseType,omitempty" yaml:"responseType,omitempty"`
}
