package client

const (
	LoadBalancerStatusType         = "loadBalancerStatus"
	LoadBalancerStatusFieldIngress = "ingress"
)

type LoadBalancerStatus struct {
	Ingress []LoadBalancerIngress `json:"ingress,omitempty"`
}
