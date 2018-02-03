package client

const (
	LocalLoginType              = "localLogin"
	LocalLoginFieldDescription  = "description"
	LocalLoginFieldPassword     = "password"
	LocalLoginFieldResponseType = "responseType"
	LocalLoginFieldTTLMillis    = "ttl"
	LocalLoginFieldUsername     = "username"
)

type LocalLogin struct {
	Description  string `json:"description,omitempty"`
	Password     string `json:"password,omitempty"`
	ResponseType string `json:"responseType,omitempty"`
	TTLMillis    *int64 `json:"ttl,omitempty"`
	Username     string `json:"username,omitempty"`
}
