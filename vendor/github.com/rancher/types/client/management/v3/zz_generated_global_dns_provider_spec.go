package client

const (
	GlobalDNSProviderSpecType                          = "globalDnsProviderSpec"
	GlobalDNSProviderSpecFieldCloudflareProviderConfig = "cloudflareProviderConfig"
	GlobalDNSProviderSpecFieldMembers                  = "members"
	GlobalDNSProviderSpecFieldRoute53ProviderConfig    = "route53ProviderConfig"
)

type GlobalDNSProviderSpec struct {
	CloudflareProviderConfig *CloudflareProviderConfig `json:"cloudflareProviderConfig,omitempty" yaml:"cloudflareProviderConfig,omitempty"`
	Members                  []Member                  `json:"members,omitempty" yaml:"members,omitempty"`
	Route53ProviderConfig    *Route53ProviderConfig    `json:"route53ProviderConfig,omitempty" yaml:"route53ProviderConfig,omitempty"`
}
