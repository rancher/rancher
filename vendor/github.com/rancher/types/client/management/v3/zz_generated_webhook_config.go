package client

const (
	WebhookConfigType     = "webhookConfig"
	WebhookConfigFieldURL = "url"
)

type WebhookConfig struct {
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
}
