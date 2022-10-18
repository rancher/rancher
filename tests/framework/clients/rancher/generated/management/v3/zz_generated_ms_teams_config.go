package client

const (
	MSTeamsConfigType          = "msTeamsConfig"
	MSTeamsConfigFieldProxyURL = "proxyUrl"
	MSTeamsConfigFieldURL      = "url"
)

type MSTeamsConfig struct {
	ProxyURL string `json:"proxyUrl,omitempty" yaml:"proxyUrl,omitempty"`
	URL      string `json:"url,omitempty" yaml:"url,omitempty"`
}
