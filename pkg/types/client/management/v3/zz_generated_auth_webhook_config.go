package client

const (
	AuthWebhookConfigType              = "authWebhookConfig"
	AuthWebhookConfigFieldCacheTimeout = "cacheTimeout"
	AuthWebhookConfigFieldConfigFile   = "configFile"
)

type AuthWebhookConfig struct {
	CacheTimeout string `json:"cacheTimeout,omitempty" yaml:"cacheTimeout,omitempty"`
	ConfigFile   string `json:"configFile,omitempty" yaml:"configFile,omitempty"`
}
