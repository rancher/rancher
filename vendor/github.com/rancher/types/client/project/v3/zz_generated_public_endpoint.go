package client

const (
	PublicEndpointType           = "publicEndpoint"
	PublicEndpointFieldAddress   = "address"
	PublicEndpointFieldNodeId    = "nodeId"
	PublicEndpointFieldPodId     = "podId"
	PublicEndpointFieldPort      = "port"
	PublicEndpointFieldProtocol  = "protocol"
	PublicEndpointFieldServiceId = "serviceId"
)

type PublicEndpoint struct {
	Address   string `json:"address,omitempty"`
	NodeId    string `json:"nodeId,omitempty"`
	PodId     string `json:"podId,omitempty"`
	Port      *int64 `json:"port,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	ServiceId string `json:"serviceId,omitempty"`
}
