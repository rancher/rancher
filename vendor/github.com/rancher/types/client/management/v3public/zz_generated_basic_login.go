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
	Description  string `json:"description,omitempty"`
	Password     string `json:"password,omitempty"`
	ResponseType string `json:"responseType,omitempty"`
	TTLMillis    *int64 `json:"ttl,omitempty"`
	Username     string `json:"username,omitempty"`
}
