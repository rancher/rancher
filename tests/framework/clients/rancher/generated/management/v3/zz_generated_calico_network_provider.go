package client

const (
	CalicoNetworkProviderType               = "calicoNetworkProvider"
	CalicoNetworkProviderFieldCloudProvider = "cloudProvider"
)

type CalicoNetworkProvider struct {
	CloudProvider string `json:"cloudProvider,omitempty" yaml:"cloudProvider,omitempty"`
}
