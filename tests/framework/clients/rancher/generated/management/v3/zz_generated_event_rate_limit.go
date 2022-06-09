package client

const (
	EventRateLimitType               = "eventRateLimit"
	EventRateLimitFieldConfiguration = "configuration"
	EventRateLimitFieldEnabled       = "enabled"
)

type EventRateLimit struct {
	Configuration map[string]interface{} `json:"configuration,omitempty" yaml:"configuration,omitempty"`
	Enabled       bool                   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}
