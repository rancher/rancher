package client

const (
	GoogleOauthConfigApplyInputType                   = "googleOauthConfigApplyInput"
	GoogleOauthConfigApplyInputFieldCode              = "code"
	GoogleOauthConfigApplyInputFieldConfigName        = "configName"
	GoogleOauthConfigApplyInputFieldEnabled           = "enabled"
	GoogleOauthConfigApplyInputFieldGoogleOauthConfig = "googleOauthConfig"
)

type GoogleOauthConfigApplyInput struct {
	Code              string             `json:"code,omitempty" yaml:"code,omitempty"`
	ConfigName        string             `json:"configName,omitempty" yaml:"configName,omitempty"`
	Enabled           bool               `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	GoogleOauthConfig *GoogleOauthConfig `json:"googleOauthConfig,omitempty" yaml:"googleOauthConfig,omitempty"`
}
