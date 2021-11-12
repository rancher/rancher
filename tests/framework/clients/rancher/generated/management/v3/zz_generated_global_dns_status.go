package client

const (
	GlobalDNSStatusType                  = "globalDnsStatus"
	GlobalDNSStatusFieldClusterEndpoints = "clusterEndpoints"
	GlobalDNSStatusFieldEndpoints        = "endpoints"
)

type GlobalDNSStatus struct {
	ClusterEndpoints map[string][]string `json:"clusterEndpoints,omitempty" yaml:"clusterEndpoints,omitempty"`
	Endpoints        []string            `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
}
