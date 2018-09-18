package client

const (
	GlobalDNSProviderSpecType                       = "globalDnsProviderSpec"
	GlobalDNSProviderSpecFieldRoute53ProviderConfig = "route53ProviderConfig"
)

type GlobalDNSProviderSpec struct {
	Route53ProviderConfig *Route53ProviderConfig `json:"route53ProviderConfig,omitempty" yaml:"route53ProviderConfig,omitempty"`
}
