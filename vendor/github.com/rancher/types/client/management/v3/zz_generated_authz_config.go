package client

const (
	AuthzConfigType         = "authzConfig"
	AuthzConfigFieldMode    = "mode"
	AuthzConfigFieldOptions = "options"
)

type AuthzConfig struct {
	Mode    string            `json:"mode,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}
