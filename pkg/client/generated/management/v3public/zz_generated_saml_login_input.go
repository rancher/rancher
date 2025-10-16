package client

const (
	SamlLoginInputType                  = "samlLoginInput"
	SamlLoginInputFieldDescription      = "description"
	SamlLoginInputFieldFinalRedirectURL = "finalRedirectUrl"
	SamlLoginInputFieldPublicKey        = "publicKey"
	SamlLoginInputFieldRequestID        = "requestId"
	SamlLoginInputFieldResponseType     = "responseType"
	SamlLoginInputFieldTTLMillis        = "ttl"
	SamlLoginInputFieldType             = "type"
)

type SamlLoginInput struct {
	Description      string `json:"description,omitempty" yaml:"description,omitempty"`
	FinalRedirectURL string `json:"finalRedirectUrl,omitempty" yaml:"finalRedirectUrl,omitempty"`
	PublicKey        string `json:"publicKey,omitempty" yaml:"publicKey,omitempty"`
	RequestID        string `json:"requestId,omitempty" yaml:"requestId,omitempty"`
	ResponseType     string `json:"responseType,omitempty" yaml:"responseType,omitempty"`
	TTLMillis        int64  `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Type             string `json:"type,omitempty" yaml:"type,omitempty"`
}
