package client

const (
	IngressLoadBalancerStatusType         = "ingressLoadBalancerStatus"
	IngressLoadBalancerStatusFieldIngress = "ingress"
)

type IngressLoadBalancerStatus struct {
	Ingress []IngressLoadBalancerIngress `json:"ingress,omitempty" yaml:"ingress,omitempty"`
}
