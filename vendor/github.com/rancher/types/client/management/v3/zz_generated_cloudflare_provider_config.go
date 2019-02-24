package client

const (
	CloudflareProviderConfigType          = "cloudflareProviderConfig"
	CloudflareProviderConfigFieldAPIEmail = "apiEmail"
	CloudflareProviderConfigFieldAPIKey   = "apiKey"
)

type CloudflareProviderConfig struct {
	APIEmail string `json:"apiEmail,omitempty" yaml:"apiEmail,omitempty"`
	APIKey   string `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
}
