package client

const (
	FlannelNetworkProviderType                = "flannelNetworkProvider"
	FlannelNetworkProviderFieldBlackholeRoute = "blackholeRoute"
	FlannelNetworkProviderFieldIface          = "iface"
)

type FlannelNetworkProvider struct {
	BlackholeRoute string `json:"blackholeRoute,omitempty" yaml:"blackholeRoute,omitempty"`
	Iface          string `json:"iface,omitempty" yaml:"iface,omitempty"`
}
