package client

const (
	L4LoadBalancerType                      = "l4LoadBalancer"
	L4LoadBalancerFieldEnabled              = "enabled"
	L4LoadBalancerFieldHealthCheckSupported = "healthCheckSupported"
	L4LoadBalancerFieldProtocolsSupported   = "protocolsSupported"
	L4LoadBalancerFieldProvider             = "provider"
)

type L4LoadBalancer struct {
	Enabled              bool     `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	HealthCheckSupported bool     `json:"healthCheckSupported,omitempty" yaml:"healthCheckSupported,omitempty"`
	ProtocolsSupported   []string `json:"protocolsSupported,omitempty" yaml:"protocolsSupported,omitempty"`
	Provider             string   `json:"provider,omitempty" yaml:"provider,omitempty"`
}
