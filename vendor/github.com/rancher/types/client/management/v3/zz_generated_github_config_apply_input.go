package client

const (
	GithubConfigApplyInputType              = "githubConfigApplyInput"
	GithubConfigApplyInputFieldCode         = "code"
	GithubConfigApplyInputFieldEnabled      = "enabled"
	GithubConfigApplyInputFieldGithubConfig = "githubConfig"
)

type GithubConfigApplyInput struct {
	Code         string        `json:"code,omitempty"`
	Enabled      *bool         `json:"enabled,omitempty"`
	GithubConfig *GithubConfig `json:"githubConfig,omitempty"`
}
