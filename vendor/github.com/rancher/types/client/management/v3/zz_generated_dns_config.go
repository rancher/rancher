package client

const (
	DNSConfigType                        = "dnsConfig"
	DNSConfigFieldLinearAutoscalerParams = "linearAutoscalerParams"
	DNSConfigFieldNodeSelector           = "nodeSelector"
	DNSConfigFieldNodelocal              = "nodelocal"
	DNSConfigFieldProvider               = "provider"
	DNSConfigFieldReverseCIDRs           = "reversecidrs"
	DNSConfigFieldStubDomains            = "stubdomains"
	DNSConfigFieldUpdateStrategy         = "updateStrategy"
	DNSConfigFieldUpstreamNameservers    = "upstreamnameservers"
)

type DNSConfig struct {
	LinearAutoscalerParams *LinearAutoscalerParams `json:"linearAutoscalerParams,omitempty" yaml:"linearAutoscalerParams,omitempty"`
	NodeSelector           map[string]string       `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Nodelocal              *Nodelocal              `json:"nodelocal,omitempty" yaml:"nodelocal,omitempty"`
	Provider               string                  `json:"provider,omitempty" yaml:"provider,omitempty"`
	ReverseCIDRs           []string                `json:"reversecidrs,omitempty" yaml:"reversecidrs,omitempty"`
	StubDomains            map[string][]string     `json:"stubdomains,omitempty" yaml:"stubdomains,omitempty"`
	UpdateStrategy         *DeploymentStrategy     `json:"updateStrategy,omitempty" yaml:"updateStrategy,omitempty"`
	UpstreamNameservers    []string                `json:"upstreamnameservers,omitempty" yaml:"upstreamnameservers,omitempty"`
}
