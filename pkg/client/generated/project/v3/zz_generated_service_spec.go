package client

const (
	ServiceSpecType                               = "serviceSpec"
	ServiceSpecFieldAllocateLoadBalancerNodePorts = "allocateLoadBalancerNodePorts"
	ServiceSpecFieldClusterIPs                    = "clusterIPs"
	ServiceSpecFieldClusterIp                     = "clusterIp"
	ServiceSpecFieldExternalIPs                   = "externalIPs"
	ServiceSpecFieldExternalTrafficPolicy         = "externalTrafficPolicy"
	ServiceSpecFieldHealthCheckNodePort           = "healthCheckNodePort"
	ServiceSpecFieldHostname                      = "hostname"
	ServiceSpecFieldIPFamilies                    = "ipFamilies"
	ServiceSpecFieldIPFamilyPolicy                = "ipFamilyPolicy"
	ServiceSpecFieldInternalTrafficPolicy         = "internalTrafficPolicy"
	ServiceSpecFieldLoadBalancerClass             = "loadBalancerClass"
	ServiceSpecFieldLoadBalancerIP                = "loadBalancerIP"
	ServiceSpecFieldLoadBalancerSourceRanges      = "loadBalancerSourceRanges"
	ServiceSpecFieldPorts                         = "ports"
	ServiceSpecFieldPublishNotReadyAddresses      = "publishNotReadyAddresses"
	ServiceSpecFieldSelector                      = "selector"
	ServiceSpecFieldServiceKind                   = "serviceKind"
	ServiceSpecFieldSessionAffinity               = "sessionAffinity"
	ServiceSpecFieldSessionAffinityConfig         = "sessionAffinityConfig"
	ServiceSpecFieldTrafficDistribution           = "trafficDistribution"
)

type ServiceSpec struct {
	AllocateLoadBalancerNodePorts *bool                  `json:"allocateLoadBalancerNodePorts,omitempty" yaml:"allocateLoadBalancerNodePorts,omitempty"`
	ClusterIPs                    []string               `json:"clusterIPs,omitempty" yaml:"clusterIPs,omitempty"`
	ClusterIp                     string                 `json:"clusterIp,omitempty" yaml:"clusterIp,omitempty"`
	ExternalIPs                   []string               `json:"externalIPs,omitempty" yaml:"externalIPs,omitempty"`
	ExternalTrafficPolicy         string                 `json:"externalTrafficPolicy,omitempty" yaml:"externalTrafficPolicy,omitempty"`
	HealthCheckNodePort           int64                  `json:"healthCheckNodePort,omitempty" yaml:"healthCheckNodePort,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	IPFamilies                    []string               `json:"ipFamilies,omitempty" yaml:"ipFamilies,omitempty"`
	IPFamilyPolicy                string                 `json:"ipFamilyPolicy,omitempty" yaml:"ipFamilyPolicy,omitempty"`
	InternalTrafficPolicy         string                 `json:"internalTrafficPolicy,omitempty" yaml:"internalTrafficPolicy,omitempty"`
	LoadBalancerClass             string                 `json:"loadBalancerClass,omitempty" yaml:"loadBalancerClass,omitempty"`
	LoadBalancerIP                string                 `json:"loadBalancerIP,omitempty" yaml:"loadBalancerIP,omitempty"`
	LoadBalancerSourceRanges      []string               `json:"loadBalancerSourceRanges,omitempty" yaml:"loadBalancerSourceRanges,omitempty"`
	Ports                         []ServicePort          `json:"ports,omitempty" yaml:"ports,omitempty"`
	PublishNotReadyAddresses      bool                   `json:"publishNotReadyAddresses,omitempty" yaml:"publishNotReadyAddresses,omitempty"`
	Selector                      map[string]string      `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceKind                   string                 `json:"serviceKind,omitempty" yaml:"serviceKind,omitempty"`
	SessionAffinity               string                 `json:"sessionAffinity,omitempty" yaml:"sessionAffinity,omitempty"`
	SessionAffinityConfig         *SessionAffinityConfig `json:"sessionAffinityConfig,omitempty" yaml:"sessionAffinityConfig,omitempty"`
	TrafficDistribution           string                 `json:"trafficDistribution,omitempty" yaml:"trafficDistribution,omitempty"`
}
