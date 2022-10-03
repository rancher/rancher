package client

const (
	RegistryType         = "registry"
	RegistryFieldConfigs = "configs"
	RegistryFieldMirrors = "mirrors"
)

type Registry struct {
	Configs map[string]RegistryConfig `json:"configs,omitempty" yaml:"configs,omitempty"`
	Mirrors map[string]Mirror         `json:"mirrors,omitempty" yaml:"mirrors,omitempty"`
}
