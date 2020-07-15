package client

const (
	GatewaySpecType          = "gatewaySpec"
	GatewaySpecFieldSelector = "selector"
	GatewaySpecFieldServers  = "servers"
)

type GatewaySpec struct {
	Selector map[string]string `json:"selector,omitempty" yaml:"selector,omitempty"`
	Servers  []Server          `json:"servers,omitempty" yaml:"servers,omitempty"`
}
