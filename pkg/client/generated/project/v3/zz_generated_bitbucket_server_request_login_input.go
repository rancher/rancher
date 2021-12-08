package client

const (
	BitbucketServerRequestLoginInputType             = "bitbucketServerRequestLoginInput"
	BitbucketServerRequestLoginInputFieldHostname    = "hostname"
	BitbucketServerRequestLoginInputFieldRedirectURL = "redirectUrl"
	BitbucketServerRequestLoginInputFieldTLS         = "tls"
)

type BitbucketServerRequestLoginInput struct {
	Hostname    string `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	RedirectURL string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	TLS         bool   `json:"tls,omitempty" yaml:"tls,omitempty"`
}
