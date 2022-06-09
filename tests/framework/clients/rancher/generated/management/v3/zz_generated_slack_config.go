package client

const (
	SlackConfigType                  = "slackConfig"
	SlackConfigFieldDefaultRecipient = "defaultRecipient"
	SlackConfigFieldProxyURL         = "proxyUrl"
	SlackConfigFieldURL              = "url"
)

type SlackConfig struct {
	DefaultRecipient string `json:"defaultRecipient,omitempty" yaml:"defaultRecipient,omitempty"`
	ProxyURL         string `json:"proxyUrl,omitempty" yaml:"proxyUrl,omitempty"`
	URL              string `json:"url,omitempty" yaml:"url,omitempty"`
}
