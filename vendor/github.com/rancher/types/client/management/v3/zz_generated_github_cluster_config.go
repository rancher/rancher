package client

const (
	GithubClusterConfigType              = "githubClusterConfig"
	GithubClusterConfigFieldClientID     = "clientId"
	GithubClusterConfigFieldClientSecret = "clientSecret"
	GithubClusterConfigFieldHost         = "host"
	GithubClusterConfigFieldRedirectURL  = "redirectUrl"
	GithubClusterConfigFieldTLS          = "tls"
)

type GithubClusterConfig struct {
	ClientID     string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Host         string `json:"host,omitempty" yaml:"host,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	TLS          bool   `json:"tls,omitempty" yaml:"tls,omitempty"`
}
