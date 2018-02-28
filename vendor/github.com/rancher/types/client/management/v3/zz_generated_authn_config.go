package client

const (
	AuthnConfigType          = "authnConfig"
	AuthnConfigFieldOptions  = "options"
	AuthnConfigFieldStrategy = "strategy"
)

type AuthnConfig struct {
	Options  map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
	Strategy string            `json:"strategy,omitempty" yaml:"strategy,omitempty"`
}
