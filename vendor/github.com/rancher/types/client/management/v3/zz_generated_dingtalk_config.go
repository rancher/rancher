package client

const (
	DingtalkConfigType          = "dingtalkConfig"
	DingtalkConfigFieldProxyURL = "proxyUrl"
	DingtalkConfigFieldSecret   = "secret"
	DingtalkConfigFieldURL      = "url"
)

type DingtalkConfig struct {
	ProxyURL string `json:"proxyUrl,omitempty" yaml:"proxyUrl,omitempty"`
	Secret   string `json:"secret,omitempty" yaml:"secret,omitempty"`
	URL      string `json:"url,omitempty" yaml:"url,omitempty"`
}
