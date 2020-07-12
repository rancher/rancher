package client

const (
	BitbucketCloudApplyInputType              = "bitbucketCloudApplyInput"
	BitbucketCloudApplyInputFieldClientID     = "clientId"
	BitbucketCloudApplyInputFieldClientSecret = "clientSecret"
	BitbucketCloudApplyInputFieldCode         = "code"
	BitbucketCloudApplyInputFieldHostname     = "hostname"
	BitbucketCloudApplyInputFieldRedirectURL  = "redirectUrl"
	BitbucketCloudApplyInputFieldTLS          = "tls"
)

type BitbucketCloudApplyInput struct {
	ClientID     string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Code         string `json:"code,omitempty" yaml:"code,omitempty"`
	Hostname     string `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	TLS          bool   `json:"tls,omitempty" yaml:"tls,omitempty"`
}
