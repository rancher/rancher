package client

const (
	ServiceStatusType              = "serviceStatus"
	ServiceStatusFieldConditions   = "conditions"
	ServiceStatusFieldLoadBalancer = "loadBalancer"
)

type ServiceStatus struct {
	Conditions   []Condition         `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	LoadBalancer *LoadBalancerStatus `json:"loadBalancer,omitempty" yaml:"loadBalancer,omitempty"`
}
