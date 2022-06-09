package client

const (
	ResourceConfigurationType           = "resourceConfiguration"
	ResourceConfigurationFieldProviders = "providers"
	ResourceConfigurationFieldResources = "resources"
)

type ResourceConfiguration struct {
	Providers []ProviderConfiguration `json:"providers,omitempty" yaml:"providers,omitempty"`
	Resources []string                `json:"resources,omitempty" yaml:"resources,omitempty"`
}
