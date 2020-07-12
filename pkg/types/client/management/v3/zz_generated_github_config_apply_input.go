package client

const (
	GithubConfigApplyInputType              = "githubConfigApplyInput"
	GithubConfigApplyInputFieldCode         = "code"
	GithubConfigApplyInputFieldEnabled      = "enabled"
	GithubConfigApplyInputFieldGithubConfig = "githubConfig"
)

type GithubConfigApplyInput struct {
	Code         string        `json:"code,omitempty" yaml:"code,omitempty"`
	Enabled      bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	GithubConfig *GithubConfig `json:"githubConfig,omitempty" yaml:"githubConfig,omitempty"`
}
