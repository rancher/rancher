package client

const (
	LoadBalancerSettingsType                = "loadBalancerSettings"
	LoadBalancerSettingsFieldConsistentHash = "consistentHash"
	LoadBalancerSettingsFieldSimple         = "simple"
)

type LoadBalancerSettings struct {
	ConsistentHash *ConsistentHashLB `json:"consistentHash,omitempty" yaml:"consistentHash,omitempty"`
	Simple         string            `json:"simple,omitempty" yaml:"simple,omitempty"`
}
