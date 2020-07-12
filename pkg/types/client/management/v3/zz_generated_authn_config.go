package client

const (
	AuthnConfigType          = "authnConfig"
	AuthnConfigFieldSANs     = "sans"
	AuthnConfigFieldStrategy = "strategy"
	AuthnConfigFieldWebhook  = "webhook"
)

type AuthnConfig struct {
	SANs     []string           `json:"sans,omitempty" yaml:"sans,omitempty"`
	Strategy string             `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	Webhook  *AuthWebhookConfig `json:"webhook,omitempty" yaml:"webhook,omitempty"`
}
