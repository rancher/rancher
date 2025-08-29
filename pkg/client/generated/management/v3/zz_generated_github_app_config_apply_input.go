package client

const (
	GithubAppConfigApplyInputType              = "githubAppConfigApplyInput"
	GithubAppConfigApplyInputFieldCode         = "code"
	GithubAppConfigApplyInputFieldEnabled      = "enabled"
	GithubAppConfigApplyInputFieldGithubConfig = "githubConfig"
)

type GithubAppConfigApplyInput struct {
	Code         string           `json:"code,omitempty" yaml:"code,omitempty"`
	Enabled      bool             `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	GithubConfig *GithubAppConfig `json:"githubConfig,omitempty" yaml:"githubConfig,omitempty"`
}
