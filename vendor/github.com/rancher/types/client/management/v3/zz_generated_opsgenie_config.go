package client

const (
	OpsgenieConfigType                  = "opsgenieConfig"
	OpsgenieConfigFieldAPIKey           = "api_key"
	OpsgenieConfigFieldDefaultRecipient = "defaultRecipient"
	OpsgenieConfigFieldProxyURL         = "proxyUrl"
	OpsgenieConfigFieldRegion           = "region"
	OpsgenieConfigFieldTags             = "tags"
)

type OpsgenieConfig struct {
	APIKey           string `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	DefaultRecipient string `json:"defaultRecipient,omitempty" yaml:"defaultRecipient,omitempty"`
	ProxyURL         string `json:"proxyUrl,omitempty" yaml:"proxyUrl,omitempty"`
	Region           string `json:"region,omitempty" yaml:"region,omitempty"`
	Tags             string `json:"tags,omitempty" yaml:"tags,omitempty"`
}
