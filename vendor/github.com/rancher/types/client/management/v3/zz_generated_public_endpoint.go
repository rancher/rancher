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
	Address   string `json:"address,omitempty" yaml:"address,omitempty"`
	NodeId    string `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	PodId     string `json:"podId,omitempty" yaml:"podId,omitempty"`
	Port      int64  `json:"port,omitempty" yaml:"port,omitempty"`
	Protocol  string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	ServiceId string `json:"serviceId,omitempty" yaml:"serviceId,omitempty"`
}
