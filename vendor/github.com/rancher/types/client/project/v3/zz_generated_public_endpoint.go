package client

const (
	PublicEndpointType           = "publicEndpoint"
	PublicEndpointFieldAddresses = "addresses"
	PublicEndpointFieldAllNodes  = "allNodes"
	PublicEndpointFieldHostname  = "hostname"
	PublicEndpointFieldIngressID = "ingressId"
	PublicEndpointFieldNodeID    = "nodeId"
	PublicEndpointFieldPath      = "path"
	PublicEndpointFieldPodID     = "podId"
	PublicEndpointFieldPort      = "port"
	PublicEndpointFieldProtocol  = "protocol"
	PublicEndpointFieldServiceID = "serviceId"
)

type PublicEndpoint struct {
	Addresses []string `json:"addresses,omitempty" yaml:"addresses,omitempty"`
	AllNodes  bool     `json:"allNodes,omitempty" yaml:"allNodes,omitempty"`
	Hostname  string   `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	IngressID string   `json:"ingressId,omitempty" yaml:"ingressId,omitempty"`
	NodeID    string   `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	Path      string   `json:"path,omitempty" yaml:"path,omitempty"`
	PodID     string   `json:"podId,omitempty" yaml:"podId,omitempty"`
	Port      int64    `json:"port,omitempty" yaml:"port,omitempty"`
	Protocol  string   `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	ServiceID string   `json:"serviceId,omitempty" yaml:"serviceId,omitempty"`
}
