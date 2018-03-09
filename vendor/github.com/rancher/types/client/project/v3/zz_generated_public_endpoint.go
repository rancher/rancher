package client

const (
	PublicEndpointType           = "publicEndpoint"
	PublicEndpointFieldAddress   = "address"
	PublicEndpointFieldHostname  = "hostname"
	PublicEndpointFieldNodeId    = "nodeId"
	PublicEndpointFieldPath      = "path"
	PublicEndpointFieldPodId     = "podId"
	PublicEndpointFieldPort      = "port"
	PublicEndpointFieldProtocol  = "protocol"
	PublicEndpointFieldServiceId = "serviceId"
)

type PublicEndpoint struct {
	Address   string `json:"address,omitempty" yaml:"address,omitempty"`
	Hostname  string `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	NodeId    string `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	Path      string `json:"path,omitempty" yaml:"path,omitempty"`
	PodId     string `json:"podId,omitempty" yaml:"podId,omitempty"`
	Port      int64  `json:"port,omitempty" yaml:"port,omitempty"`
	Protocol  string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	ServiceId string `json:"serviceId,omitempty" yaml:"serviceId,omitempty"`
}
