package client

const (
	OAuthAuthorizationInfoType              = "oAuthAuthorizationInfo"
	OAuthAuthorizationInfoFieldClientID     = "clientId"
	OAuthAuthorizationInfoFieldClientSecret = "clientSecret"
	OAuthAuthorizationInfoFieldRedirectURL  = "redirectUrl"
)

type OAuthAuthorizationInfo struct {
	ClientID     string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
}
