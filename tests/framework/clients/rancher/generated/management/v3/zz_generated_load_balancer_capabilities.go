package client

const (
	LoadBalancerCapabilitiesType                      = "loadBalancerCapabilities"
	LoadBalancerCapabilitiesFieldEnabled              = "enabled"
	LoadBalancerCapabilitiesFieldHealthCheckSupported = "healthCheckSupported"
	LoadBalancerCapabilitiesFieldProtocolsSupported   = "protocolsSupported"
	LoadBalancerCapabilitiesFieldProvider             = "provider"
)

type LoadBalancerCapabilities struct {
	Enabled              *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	HealthCheckSupported bool     `json:"healthCheckSupported,omitempty" yaml:"healthCheckSupported,omitempty"`
	ProtocolsSupported   []string `json:"protocolsSupported,omitempty" yaml:"protocolsSupported,omitempty"`
	Provider             string   `json:"provider,omitempty" yaml:"provider,omitempty"`
}
