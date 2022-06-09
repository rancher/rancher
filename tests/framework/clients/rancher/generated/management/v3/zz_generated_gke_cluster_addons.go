package client

const (
	GKEClusterAddonsType                          = "gkeClusterAddons"
	GKEClusterAddonsFieldHTTPLoadBalancing        = "httpLoadBalancing"
	GKEClusterAddonsFieldHorizontalPodAutoscaling = "horizontalPodAutoscaling"
	GKEClusterAddonsFieldNetworkPolicyConfig      = "networkPolicyConfig"
)

type GKEClusterAddons struct {
	HTTPLoadBalancing        bool `json:"httpLoadBalancing,omitempty" yaml:"httpLoadBalancing,omitempty"`
	HorizontalPodAutoscaling bool `json:"horizontalPodAutoscaling,omitempty" yaml:"horizontalPodAutoscaling,omitempty"`
	NetworkPolicyConfig      bool `json:"networkPolicyConfig,omitempty" yaml:"networkPolicyConfig,omitempty"`
}
