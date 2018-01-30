package client

const (
	GithubConfigApplyInputType                  = "githubConfigApplyInput"
	GithubConfigApplyInputFieldEnabled          = "enabled"
	GithubConfigApplyInputFieldGithubConfig     = "githubConfig"
	GithubConfigApplyInputFieldGithubCredential = "githubCredential"
)

type GithubConfigApplyInput struct {
	Enabled          *bool             `json:"enabled,omitempty"`
	GithubConfig     *GithubConfig     `json:"githubConfig,omitempty"`
	GithubCredential *GithubCredential `json:"githubCredential,omitempty"`
}
