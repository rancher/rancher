package client

const (
	CapabilitiesType                          = "capabilities"
	CapabilitiesFieldIngressCapabilities      = "ingressCapabilities"
	CapabilitiesFieldLoadBalancerCapabilities = "loadBalancerCapabilities"
	CapabilitiesFieldNodePoolScalingSupported = "nodePoolScalingSupported"
	CapabilitiesFieldNodePortRange            = "nodePortRange"
	CapabilitiesFieldPspEnabled               = "pspEnabled"
	CapabilitiesFieldTaintSupport             = "taintSupport"
)

type Capabilities struct {
	IngressCapabilities      []IngressCapabilities     `json:"ingressCapabilities,omitempty" yaml:"ingressCapabilities,omitempty"`
	LoadBalancerCapabilities *LoadBalancerCapabilities `json:"loadBalancerCapabilities,omitempty" yaml:"loadBalancerCapabilities,omitempty"`
	NodePoolScalingSupported bool                      `json:"nodePoolScalingSupported,omitempty" yaml:"nodePoolScalingSupported,omitempty"`
	NodePortRange            string                    `json:"nodePortRange,omitempty" yaml:"nodePortRange,omitempty"`
	PspEnabled               bool                      `json:"pspEnabled,omitempty" yaml:"pspEnabled,omitempty"`
	TaintSupport             *bool                     `json:"taintSupport,omitempty" yaml:"taintSupport,omitempty"`
}
