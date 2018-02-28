package client

const (
	PagerdutyConfigType            = "pagerdutyConfig"
	PagerdutyConfigFieldServiceKey = "serviceKey"
)

type PagerdutyConfig struct {
	ServiceKey string `json:"serviceKey,omitempty" yaml:"serviceKey,omitempty"`
}
