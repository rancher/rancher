package client

const (
	CloudflareProviderConfigType              = "cloudflareProviderConfig"
	CloudflareProviderConfigFieldAPIEmail     = "apiEmail"
	CloudflareProviderConfigFieldAPIKey       = "apiKey"
	CloudflareProviderConfigFieldProxySetting = "proxySetting"
)

type CloudflareProviderConfig struct {
	APIEmail     string `json:"apiEmail,omitempty" yaml:"apiEmail,omitempty"`
	APIKey       string `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	ProxySetting *bool  `json:"proxySetting,omitempty" yaml:"proxySetting,omitempty"`
}
