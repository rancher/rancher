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
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	Host         string `json:"host,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty"`
	TLS          bool   `json:"tls,omitempty"`
}
