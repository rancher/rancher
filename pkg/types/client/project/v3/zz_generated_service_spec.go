package client

const (
	ServiceSpecType                          = "serviceSpec"
	ServiceSpecFieldClusterIp                = "clusterIp"
	ServiceSpecFieldExternalIPs              = "externalIPs"
	ServiceSpecFieldExternalTrafficPolicy    = "externalTrafficPolicy"
	ServiceSpecFieldHealthCheckNodePort      = "healthCheckNodePort"
	ServiceSpecFieldHostname                 = "hostname"
	ServiceSpecFieldIPFamily                 = "ipFamily"
	ServiceSpecFieldLoadBalancerIP           = "loadBalancerIP"
	ServiceSpecFieldLoadBalancerSourceRanges = "loadBalancerSourceRanges"
	ServiceSpecFieldPorts                    = "ports"
	ServiceSpecFieldPublishNotReadyAddresses = "publishNotReadyAddresses"
	ServiceSpecFieldSelector                 = "selector"
	ServiceSpecFieldServiceKind              = "serviceKind"
	ServiceSpecFieldSessionAffinity          = "sessionAffinity"
	ServiceSpecFieldSessionAffinityConfig    = "sessionAffinityConfig"
	ServiceSpecFieldTopologyKeys             = "topologyKeys"
)

type ServiceSpec struct {
	ClusterIp                string                 `json:"clusterIp,omitempty" yaml:"clusterIp,omitempty"`
	ExternalIPs              []string               `json:"externalIPs,omitempty" yaml:"externalIPs,omitempty"`
	ExternalTrafficPolicy    string                 `json:"externalTrafficPolicy,omitempty" yaml:"externalTrafficPolicy,omitempty"`
	HealthCheckNodePort      int64                  `json:"healthCheckNodePort,omitempty" yaml:"healthCheckNodePort,omitempty"`
	Hostname                 string                 `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	IPFamily                 string                 `json:"ipFamily,omitempty" yaml:"ipFamily,omitempty"`
	LoadBalancerIP           string                 `json:"loadBalancerIP,omitempty" yaml:"loadBalancerIP,omitempty"`
	LoadBalancerSourceRanges []string               `json:"loadBalancerSourceRanges,omitempty" yaml:"loadBalancerSourceRanges,omitempty"`
	Ports                    []ServicePort          `json:"ports,omitempty" yaml:"ports,omitempty"`
	PublishNotReadyAddresses bool                   `json:"publishNotReadyAddresses,omitempty" yaml:"publishNotReadyAddresses,omitempty"`
	Selector                 map[string]string      `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceKind              string                 `json:"serviceKind,omitempty" yaml:"serviceKind,omitempty"`
	SessionAffinity          string                 `json:"sessionAffinity,omitempty" yaml:"sessionAffinity,omitempty"`
	SessionAffinityConfig    *SessionAffinityConfig `json:"sessionAffinityConfig,omitempty" yaml:"sessionAffinityConfig,omitempty"`
	TopologyKeys             []string               `json:"topologyKeys,omitempty" yaml:"topologyKeys,omitempty"`
}
