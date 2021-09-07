package client

const (
	GoogleOauthConfigTestOutputType             = "googleOauthConfigTestOutput"
	GoogleOauthConfigTestOutputFieldRedirectURL = "redirectUrl"
)

type GoogleOauthConfigTestOutput struct {
	RedirectURL string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
}
