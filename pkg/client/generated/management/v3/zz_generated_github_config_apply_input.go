package client

const (
	GithubConfigApplyInputType              = "githubConfigApplyInput"
	GithubConfigApplyInputFieldCode         = "code"
	GithubConfigApplyInputFieldConfigName   = "configName"
	GithubConfigApplyInputFieldEnabled      = "enabled"
	GithubConfigApplyInputFieldGithubConfig = "githubConfig"
)

type GithubConfigApplyInput struct {
	Code         string        `json:"code,omitempty" yaml:"code,omitempty"`
	ConfigName   string        `json:"configName,omitempty" yaml:"configName,omitempty"`
	Enabled      bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	GithubConfig *GithubConfig `json:"githubConfig,omitempty" yaml:"githubConfig,omitempty"`
}
