package client

const (
	FlannelNetworkProviderType       = "flannelNetworkProvider"
	FlannelNetworkProviderFieldIface = "iface"
)

type FlannelNetworkProvider struct {
	Iface string `json:"iface,omitempty" yaml:"iface,omitempty"`
}
