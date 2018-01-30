package client

const (
	GithubConfigTestInputType              = "githubConfigTestInput"
	GithubConfigTestInputFieldEnabled      = "enabled"
	GithubConfigTestInputFieldGithubConfig = "githubConfig"
)

type GithubConfigTestInput struct {
	Enabled      *bool         `json:"enabled,omitempty"`
	GithubConfig *GithubConfig `json:"githubConfig,omitempty"`
}
