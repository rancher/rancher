package client

const (
	GlobalDNSProviderSpecType                          = "globalDnsProviderSpec"
	GlobalDNSProviderSpecFieldAlidnsProviderConfig     = "alidnsProviderConfig"
	GlobalDNSProviderSpecFieldCloudflareProviderConfig = "cloudflareProviderConfig"
	GlobalDNSProviderSpecFieldLinodeProviderConfig     = "linodeProviderConfig"
	GlobalDNSProviderSpecFieldMembers                  = "members"
	GlobalDNSProviderSpecFieldRootDomain               = "rootDomain"
	GlobalDNSProviderSpecFieldRoute53ProviderConfig    = "route53ProviderConfig"
)

type GlobalDNSProviderSpec struct {
	AlidnsProviderConfig     *AlidnsProviderConfig     `json:"alidnsProviderConfig,omitempty" yaml:"alidnsProviderConfig,omitempty"`
	CloudflareProviderConfig *CloudflareProviderConfig `json:"cloudflareProviderConfig,omitempty" yaml:"cloudflareProviderConfig,omitempty"`
	LinodeProviderConfig     *LinodeProviderConfig     `json:"linodeProviderConfig,omitempty" yaml:"linodeProviderConfig,omitempty"`
	Members                  []Member                  `json:"members,omitempty" yaml:"members,omitempty"`
	RootDomain               string                    `json:"rootDomain,omitempty" yaml:"rootDomain,omitempty"`
	Route53ProviderConfig    *Route53ProviderConfig    `json:"route53ProviderConfig,omitempty" yaml:"route53ProviderConfig,omitempty"`
}
