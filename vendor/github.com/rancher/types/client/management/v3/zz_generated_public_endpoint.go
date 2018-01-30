package client

const (
	PublicEndpointType          = "publicEndpoint"
	PublicEndpointFieldAddress  = "address"
	PublicEndpointFieldNode     = "node"
	PublicEndpointFieldPod      = "pod"
	PublicEndpointFieldPort     = "port"
	PublicEndpointFieldProtocol = "protocol"
	PublicEndpointFieldService  = "service"
)

type PublicEndpoint struct {
	Address  string `json:"address,omitempty"`
	Node     string `json:"node,omitempty"`
	Pod      string `json:"pod,omitempty"`
	Port     *int64 `json:"port,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Service  string `json:"service,omitempty"`
}
