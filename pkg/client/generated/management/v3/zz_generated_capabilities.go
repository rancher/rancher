package client

const (
	CapabilitiesType                          = "capabilities"
	CapabilitiesFieldIngressCapabilities      = "ingressCapabilities"
	CapabilitiesFieldLoadBalancerCapabilities = "loadBalancerCapabilities"
	CapabilitiesFieldNodePortRange            = "nodePortRange"
	CapabilitiesFieldTaintSupport             = "taintSupport"
)

type Capabilities struct {
	IngressCapabilities      []IngressCapabilities     `json:"ingressCapabilities,omitempty" yaml:"ingressCapabilities,omitempty"`
	LoadBalancerCapabilities *LoadBalancerCapabilities `json:"loadBalancerCapabilities,omitempty" yaml:"loadBalancerCapabilities,omitempty"`
	NodePortRange            string                    `json:"nodePortRange,omitempty" yaml:"nodePortRange,omitempty"`
	TaintSupport             *bool                     `json:"taintSupport,omitempty" yaml:"taintSupport,omitempty"`
}
