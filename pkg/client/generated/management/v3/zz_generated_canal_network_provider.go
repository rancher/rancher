package client

const (
	CanalNetworkProviderType                = "canalNetworkProvider"
	CanalNetworkProviderFieldBlackholeRoute = "blackholeRoute"
	CanalNetworkProviderFieldIface          = "iface"
)

type CanalNetworkProvider struct {
	BlackholeRoute string `json:"blackholeRoute,omitempty" yaml:"blackholeRoute,omitempty"`
	Iface          string `json:"iface,omitempty" yaml:"iface,omitempty"`
}
