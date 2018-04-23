package client

const (
	GitAppConfigType              = "gitAppConfig"
	GitAppConfigFieldClientID     = "clientId"
	GitAppConfigFieldClientSecret = "clientSecret"
	GitAppConfigFieldHost         = "host"
	GitAppConfigFieldRedirectURL  = "redirectUrl"
	GitAppConfigFieldTLS          = "tls"
)

type GitAppConfig struct {
	ClientID     string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Host         string `json:"host,omitempty" yaml:"host,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	TLS          bool   `json:"tls,omitempty" yaml:"tls,omitempty"`
}
