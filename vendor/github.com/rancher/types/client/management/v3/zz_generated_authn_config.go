package client

const (
	AuthnConfigType          = "authnConfig"
	AuthnConfigFieldOptions  = "options"
	AuthnConfigFieldSANs     = "sans"
	AuthnConfigFieldStrategy = "strategy"
)

type AuthnConfig struct {
	Options  map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
	SANs     []string          `json:"sans,omitempty" yaml:"sans,omitempty"`
	Strategy string            `json:"strategy,omitempty" yaml:"strategy,omitempty"`
}
