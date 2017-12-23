package client

const (
	ServiceSpecType                          = "serviceSpec"
	ServiceSpecFieldClusterIp                = "clusterIp"
	ServiceSpecFieldExternalIPs              = "externalIPs"
	ServiceSpecFieldExternalTrafficPolicy    = "externalTrafficPolicy"
	ServiceSpecFieldHealthCheckNodePort      = "healthCheckNodePort"
	ServiceSpecFieldHostname                 = "hostname"
	ServiceSpecFieldLoadBalancerIP           = "loadBalancerIP"
	ServiceSpecFieldLoadBalancerSourceRanges = "loadBalancerSourceRanges"
	ServiceSpecFieldPorts                    = "ports"
	ServiceSpecFieldPublishNotReadyAddresses = "publishNotReadyAddresses"
	ServiceSpecFieldSelector                 = "selector"
	ServiceSpecFieldServiceKind              = "serviceKind"
	ServiceSpecFieldSessionAffinity          = "sessionAffinity"
	ServiceSpecFieldSessionAffinityConfig    = "sessionAffinityConfig"
)

type ServiceSpec struct {
	ClusterIp                string                 `json:"clusterIp,omitempty"`
	ExternalIPs              []string               `json:"externalIPs,omitempty"`
	ExternalTrafficPolicy    string                 `json:"externalTrafficPolicy,omitempty"`
	HealthCheckNodePort      *int64                 `json:"healthCheckNodePort,omitempty"`
	Hostname                 string                 `json:"hostname,omitempty"`
	LoadBalancerIP           string                 `json:"loadBalancerIP,omitempty"`
	LoadBalancerSourceRanges []string               `json:"loadBalancerSourceRanges,omitempty"`
	Ports                    []ServicePort          `json:"ports,omitempty"`
	PublishNotReadyAddresses *bool                  `json:"publishNotReadyAddresses,omitempty"`
	Selector                 map[string]string      `json:"selector,omitempty"`
	ServiceKind              string                 `json:"serviceKind,omitempty"`
	SessionAffinity          string                 `json:"sessionAffinity,omitempty"`
	SessionAffinityConfig    *SessionAffinityConfig `json:"sessionAffinityConfig,omitempty"`
}
