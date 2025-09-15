package client

const (
	AliAddonType        = "aliAddon"
	AliAddonFieldConfig = "config"
	AliAddonFieldName   = "name"
)

type AliAddon struct {
	Config string `json:"config,omitempty" yaml:"config,omitempty"`
	Name   string `json:"name,omitempty" yaml:"name,omitempty"`
}
