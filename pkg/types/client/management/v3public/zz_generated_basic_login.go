package client

const (
	BasicLoginType              = "basicLogin"
	BasicLoginFieldDescription  = "description"
	BasicLoginFieldPassword     = "password"
	BasicLoginFieldResponseType = "responseType"
	BasicLoginFieldTTLMillis    = "ttl"
	BasicLoginFieldUsername     = "username"
)

type BasicLogin struct {
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	Password     string `json:"password,omitempty" yaml:"password,omitempty"`
	ResponseType string `json:"responseType,omitempty" yaml:"responseType,omitempty"`
	TTLMillis    int64  `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Username     string `json:"username,omitempty" yaml:"username,omitempty"`
}
