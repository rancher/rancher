package client

const (
	LoadBalancerIngressType          = "loadBalancerIngress"
	LoadBalancerIngressFieldHostname = "hostname"
	LoadBalancerIngressFieldIP       = "ip"
)

type LoadBalancerIngress struct {
	Hostname string `json:"hostname,omitempty"`
	IP       string `json:"ip,omitempty"`
}
