package client

const (
	AddonType        = "addon"
	AddonFieldConfig = "config"
	AddonFieldName   = "name"
)

type Addon struct {
	Config string `json:"config,omitempty" yaml:"config,omitempty"`
	Name   string `json:"name,omitempty" yaml:"name,omitempty"`
}
