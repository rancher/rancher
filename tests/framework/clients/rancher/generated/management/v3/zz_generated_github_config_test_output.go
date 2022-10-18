package client

const (
	GithubConfigTestOutputType             = "githubConfigTestOutput"
	GithubConfigTestOutputFieldRedirectURL = "redirectUrl"
)

type GithubConfigTestOutput struct {
	RedirectURL string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
}
