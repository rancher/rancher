package client

const (
	GoogleOauthLoginType              = "googleOauthLogin"
	GoogleOauthLoginFieldAccessToken  = "access_token"
	GoogleOauthLoginFieldCode         = "code"
	GoogleOauthLoginFieldDescription  = "description"
	GoogleOauthLoginFieldResponseType = "responseType"
	GoogleOauthLoginFieldTTLMillis    = "ttl"
)

type GoogleOauthLogin struct {
	AccessToken  string `json:"access_token,omitempty" yaml:"access_token,omitempty"`
	Code         string `json:"code,omitempty" yaml:"code,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	ResponseType string `json:"responseType,omitempty" yaml:"responseType,omitempty"`
	TTLMillis    int64  `json:"ttl,omitempty" yaml:"ttl,omitempty"`
}
