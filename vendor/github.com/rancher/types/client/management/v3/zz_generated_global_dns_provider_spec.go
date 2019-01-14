package client

const (
	GlobalDNSProviderSpecType                       = "globalDnsProviderSpec"
	GlobalDNSProviderSpecFieldMembers               = "members"
	GlobalDNSProviderSpecFieldRoute53ProviderConfig = "route53ProviderConfig"
)

type GlobalDNSProviderSpec struct {
	Members               []Member               `json:"members,omitempty" yaml:"members,omitempty"`
	Route53ProviderConfig *Route53ProviderConfig `json:"route53ProviderConfig,omitempty" yaml:"route53ProviderConfig,omitempty"`
}
