package client

const (
	CapabilitiesType                          = "capabilities"
	CapabilitiesFieldIngressControllers       = "ingressControllers"
	CapabilitiesFieldL4LoadBalancer           = "l4loadBalancer"
	CapabilitiesFieldNodePoolScalingSupported = "nodePoolScalingSupported"
	CapabilitiesFieldNodePortRange            = "nodePortRange"
)

type Capabilities struct {
	IngressControllers       []IngressController `json:"ingressControllers,omitempty" yaml:"ingressControllers,omitempty"`
	L4LoadBalancer           *L4LoadBalancer     `json:"l4loadBalancer,omitempty" yaml:"l4loadBalancer,omitempty"`
	NodePoolScalingSupported bool                `json:"nodePoolScalingSupported,omitempty" yaml:"nodePoolScalingSupported,omitempty"`
	NodePortRange            string              `json:"nodePortRange,omitempty" yaml:"nodePortRange,omitempty"`
}
