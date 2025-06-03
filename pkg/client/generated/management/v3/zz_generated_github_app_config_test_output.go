package client

const (
	GithubAppConfigTestOutputType             = "githubAppConfigTestOutput"
	GithubAppConfigTestOutputFieldRedirectURL = "redirectUrl"
)

type GithubAppConfigTestOutput struct {
	RedirectURL string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
}
