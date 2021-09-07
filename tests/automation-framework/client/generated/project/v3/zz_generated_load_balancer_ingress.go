package client

const (
	LoadBalancerIngressType          = "loadBalancerIngress"
	LoadBalancerIngressFieldHostname = "hostname"
	LoadBalancerIngressFieldIP       = "ip"
	LoadBalancerIngressFieldPorts    = "ports"
)

type LoadBalancerIngress struct {
	Hostname string       `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	IP       string       `json:"ip,omitempty" yaml:"ip,omitempty"`
	Ports    []PortStatus `json:"ports,omitempty" yaml:"ports,omitempty"`
}
