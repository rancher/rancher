package client

const (
	DNSConfigType                     = "dnsConfig"
	DNSConfigFieldNodeSelector        = "nodeSelector"
	DNSConfigFieldNodelocal           = "nodelocal"
	DNSConfigFieldProvider            = "provider"
	DNSConfigFieldReverseCIDRs        = "reversecidrs"
	DNSConfigFieldStubDomains         = "stubdomains"
	DNSConfigFieldUpstreamNameservers = "upstreamnameservers"
)

type DNSConfig struct {
	NodeSelector        map[string]string   `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Nodelocal           *Nodelocal          `json:"nodelocal,omitempty" yaml:"nodelocal,omitempty"`
	Provider            string              `json:"provider,omitempty" yaml:"provider,omitempty"`
	ReverseCIDRs        []string            `json:"reversecidrs,omitempty" yaml:"reversecidrs,omitempty"`
	StubDomains         map[string][]string `json:"stubdomains,omitempty" yaml:"stubdomains,omitempty"`
	UpstreamNameservers []string            `json:"upstreamnameservers,omitempty" yaml:"upstreamnameservers,omitempty"`
}
