package client

const (
	NetworkConfigType                        = "networkConfig"
	NetworkConfigFieldCalicoNetworkProvider  = "calicoNetworkProvider"
	NetworkConfigFieldCanalNetworkProvider   = "canalNetworkProvider"
	NetworkConfigFieldFlannelNetworkProvider = "flannelNetworkProvider"
	NetworkConfigFieldOptions                = "options"
	NetworkConfigFieldPlugin                 = "plugin"
)

type NetworkConfig struct {
	CalicoNetworkProvider  *CalicoNetworkProvider  `json:"calicoNetworkProvider,omitempty" yaml:"calicoNetworkProvider,omitempty"`
	CanalNetworkProvider   *CanalNetworkProvider   `json:"canalNetworkProvider,omitempty" yaml:"canalNetworkProvider,omitempty"`
	FlannelNetworkProvider *FlannelNetworkProvider `json:"flannelNetworkProvider,omitempty" yaml:"flannelNetworkProvider,omitempty"`
	Options                map[string]string       `json:"options,omitempty" yaml:"options,omitempty"`
	Plugin                 string                  `json:"plugin,omitempty" yaml:"plugin,omitempty"`
}
