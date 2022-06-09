package client

const (
	OIDCLoginType              = "oidcLogin"
	OIDCLoginFieldCode         = "code"
	OIDCLoginFieldDescription  = "description"
	OIDCLoginFieldResponseType = "responseType"
	OIDCLoginFieldTTLMillis    = "ttl"
)

type OIDCLogin struct {
	Code         string `json:"code,omitempty" yaml:"code,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	ResponseType string `json:"responseType,omitempty" yaml:"responseType,omitempty"`
	TTLMillis    int64  `json:"ttl,omitempty" yaml:"ttl,omitempty"`
}
