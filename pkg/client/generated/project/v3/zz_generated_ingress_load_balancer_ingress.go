package client

const (
	IngressLoadBalancerIngressType          = "ingressLoadBalancerIngress"
	IngressLoadBalancerIngressFieldHostname = "hostname"
	IngressLoadBalancerIngressFieldIP       = "ip"
	IngressLoadBalancerIngressFieldPorts    = "ports"
)

type IngressLoadBalancerIngress struct {
	Hostname string              `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	IP       string              `json:"ip,omitempty" yaml:"ip,omitempty"`
	Ports    []IngressPortStatus `json:"ports,omitempty" yaml:"ports,omitempty"`
}
