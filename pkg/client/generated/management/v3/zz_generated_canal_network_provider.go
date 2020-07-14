package client

const (
	CanalNetworkProviderType       = "canalNetworkProvider"
	CanalNetworkProviderFieldIface = "iface"
)

type CanalNetworkProvider struct {
	Iface string `json:"iface,omitempty" yaml:"iface,omitempty"`
}
