package client

const (
	CloudProviderType             = "cloudProvider"
	CloudProviderFieldCloudConfig = "cloudConfig"
	CloudProviderFieldName        = "name"
)

type CloudProvider struct {
	CloudConfig map[string]string `json:"cloudConfig,omitempty" yaml:"cloudConfig,omitempty"`
	Name        string            `json:"name,omitempty" yaml:"name,omitempty"`
}
