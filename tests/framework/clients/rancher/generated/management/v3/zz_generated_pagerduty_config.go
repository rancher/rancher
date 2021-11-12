package client

const (
	PagerdutyConfigType            = "pagerdutyConfig"
	PagerdutyConfigFieldProxyURL   = "proxyUrl"
	PagerdutyConfigFieldServiceKey = "serviceKey"
)

type PagerdutyConfig struct {
	ProxyURL   string `json:"proxyUrl,omitempty" yaml:"proxyUrl,omitempty"`
	ServiceKey string `json:"serviceKey,omitempty" yaml:"serviceKey,omitempty"`
}
