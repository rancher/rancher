package client

const (
	GlobalDNSStatusType           = "globalDnsStatus"
	GlobalDNSStatusFieldEndpoints = "endpoints"
)

type GlobalDNSStatus struct {
	Endpoints []string `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
}
