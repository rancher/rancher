package client

const (
	CloudflareProviderConfigType                   = "cloudflareProviderConfig"
	CloudflareProviderConfigFieldAPIEmail          = "apiEmail"
	CloudflareProviderConfigFieldAPIKey            = "apiKey"
	CloudflareProviderConfigFieldAdditionalOptions = "additionalOptions"
	CloudflareProviderConfigFieldProxySetting      = "proxySetting"
)

type CloudflareProviderConfig struct {
	APIEmail          string            `json:"apiEmail,omitempty" yaml:"apiEmail,omitempty"`
	APIKey            string            `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	AdditionalOptions map[string]string `json:"additionalOptions,omitempty" yaml:"additionalOptions,omitempty"`
	ProxySetting      *bool             `json:"proxySetting,omitempty" yaml:"proxySetting,omitempty"`
}
