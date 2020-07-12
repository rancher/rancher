package client

const (
	GithubApplyInputType              = "githubApplyInput"
	GithubApplyInputFieldClientID     = "clientId"
	GithubApplyInputFieldClientSecret = "clientSecret"
	GithubApplyInputFieldCode         = "code"
	GithubApplyInputFieldHostname     = "hostname"
	GithubApplyInputFieldInheritAuth  = "inheritAuth"
	GithubApplyInputFieldRedirectURL  = "redirectUrl"
	GithubApplyInputFieldTLS          = "tls"
)

type GithubApplyInput struct {
	ClientID     string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Code         string `json:"code,omitempty" yaml:"code,omitempty"`
	Hostname     string `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	InheritAuth  bool   `json:"inheritAuth,omitempty" yaml:"inheritAuth,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	TLS          bool   `json:"tls,omitempty" yaml:"tls,omitempty"`
}
