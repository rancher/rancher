package client

const (
	GoogleOauthConfigApplyInputType                   = "googleOauthConfigApplyInput"
	GoogleOauthConfigApplyInputFieldCode              = "code"
	GoogleOauthConfigApplyInputFieldEnabled           = "enabled"
	GoogleOauthConfigApplyInputFieldGoogleOauthConfig = "googleOauthConfig"
)

type GoogleOauthConfigApplyInput struct {
	Code              string             `json:"code,omitempty" yaml:"code,omitempty"`
	Enabled           bool               `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	GoogleOauthConfig *GoogleOauthConfig `json:"googleOauthConfig,omitempty" yaml:"googleOauthConfig,omitempty"`
}
