package client

const (
	HarvesterCloudProviderType             = "harvesterCloudProvider"
	HarvesterCloudProviderFieldCloudConfig = "cloudConfig"
)

type HarvesterCloudProvider struct {
	CloudConfig string `json:"cloudConfig,omitempty" yaml:"cloudConfig,omitempty"`
}
