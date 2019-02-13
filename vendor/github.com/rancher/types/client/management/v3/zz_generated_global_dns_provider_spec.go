package client

const (
	GlobalDNSProviderSpecType                          = "globalDnsProviderSpec"
	GlobalDNSProviderSpecFieldAlidnsProviderConfig     = "alidnsProviderConfig"
	GlobalDNSProviderSpecFieldCloudflareProviderConfig = "cloudflareProviderConfig"
	GlobalDNSProviderSpecFieldMembers                  = "members"
	GlobalDNSProviderSpecFieldRoute53ProviderConfig    = "route53ProviderConfig"
)

type GlobalDNSProviderSpec struct {
	AlidnsProviderConfig     *AlidnsProviderConfig     `json:"alidnsProviderConfig,omitempty" yaml:"alidnsProviderConfig,omitempty"`
	CloudflareProviderConfig *CloudflareProviderConfig `json:"cloudflareProviderConfig,omitempty" yaml:"cloudflareProviderConfig,omitempty"`
	Members                  []Member                  `json:"members,omitempty" yaml:"members,omitempty"`
	Route53ProviderConfig    *Route53ProviderConfig    `json:"route53ProviderConfig,omitempty" yaml:"route53ProviderConfig,omitempty"`
}
