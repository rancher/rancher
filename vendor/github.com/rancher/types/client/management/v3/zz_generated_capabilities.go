package client

const (
	CapabilitiesType                          = "capabilities"
	CapabilitiesFieldIngressCapabilities      = "ingressCapabilities"
	CapabilitiesFieldLoadBalancerCapabilities = "loadBalancerCapabilities"
	CapabilitiesFieldNodePoolScalingSupported = "nodePoolScalingSupported"
	CapabilitiesFieldNodePortRange            = "nodePortRange"
)

type Capabilities struct {
	IngressCapabilities      []IngressCapabilities     `json:"ingressCapabilities,omitempty" yaml:"ingressCapabilities,omitempty"`
	LoadBalancerCapabilities *LoadBalancerCapabilities `json:"loadBalancerCapabilities,omitempty" yaml:"loadBalancerCapabilities,omitempty"`
	NodePoolScalingSupported bool                      `json:"nodePoolScalingSupported,omitempty" yaml:"nodePoolScalingSupported,omitempty"`
	NodePortRange            string                    `json:"nodePortRange,omitempty" yaml:"nodePortRange,omitempty"`
}
