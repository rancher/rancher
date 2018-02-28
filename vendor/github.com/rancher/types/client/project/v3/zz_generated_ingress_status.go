package client

const (
	IngressStatusType              = "ingressStatus"
	IngressStatusFieldLoadBalancer = "loadBalancer"
)

type IngressStatus struct {
	LoadBalancer *LoadBalancerStatus `json:"loadBalancer,omitempty" yaml:"loadBalancer,omitempty"`
}
