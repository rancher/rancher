package client

const (
	PublicEndpointType           = "publicEndpoint"
	PublicEndpointFieldAddresses = "addresses"
	PublicEndpointFieldAllNodes  = "allNodes"
	PublicEndpointFieldHostname  = "hostname"
	PublicEndpointFieldIngressId = "ingressId"
	PublicEndpointFieldNodeId    = "nodeId"
	PublicEndpointFieldPath      = "path"
	PublicEndpointFieldPodId     = "podId"
	PublicEndpointFieldPort      = "port"
	PublicEndpointFieldProtocol  = "protocol"
	PublicEndpointFieldServiceId = "serviceId"
)

type PublicEndpoint struct {
	Addresses []string `json:"addresses,omitempty" yaml:"addresses,omitempty"`
	AllNodes  bool     `json:"allNodes,omitempty" yaml:"allNodes,omitempty"`
	Hostname  string   `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	IngressId string   `json:"ingressId,omitempty" yaml:"ingressId,omitempty"`
	NodeId    string   `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	Path      string   `json:"path,omitempty" yaml:"path,omitempty"`
	PodId     string   `json:"podId,omitempty" yaml:"podId,omitempty"`
	Port      int64    `json:"port,omitempty" yaml:"port,omitempty"`
	Protocol  string   `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	ServiceId string   `json:"serviceId,omitempty" yaml:"serviceId,omitempty"`
}
