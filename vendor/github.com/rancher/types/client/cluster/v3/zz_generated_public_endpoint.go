package client

const (
	PublicEndpointType             = "publicEndpoint"
	PublicEndpointFieldAddress     = "address"
	PublicEndpointFieldNodeName    = "node"
	PublicEndpointFieldPodName     = "pod"
	PublicEndpointFieldPort        = "port"
	PublicEndpointFieldProtocol    = "protocol"
	PublicEndpointFieldServiceName = "service"
)

type PublicEndpoint struct {
	Address     string `json:"address,omitempty"`
	NodeName    string `json:"node,omitempty"`
	PodName     string `json:"pod,omitempty"`
	Port        *int64 `json:"port,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	ServiceName string `json:"service,omitempty"`
}
