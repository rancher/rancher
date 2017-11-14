package client

const (
	AuthConfigType          = "authConfig"
	AuthConfigFieldOptions  = "options"
	AuthConfigFieldStrategy = "strategy"
)

type AuthConfig struct {
	Options  map[string]string `json:"options,omitempty"`
	Strategy string            `json:"strategy,omitempty"`
}
