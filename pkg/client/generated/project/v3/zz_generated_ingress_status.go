package client

const (
	IngressStatusType              = "ingressStatus"
	IngressStatusFieldLoadBalancer = "loadBalancer"
)

type IngressStatus struct {
	LoadBalancer *IngressLoadBalancerStatus `json:"loadBalancer,omitempty" yaml:"loadBalancer,omitempty"`
}
