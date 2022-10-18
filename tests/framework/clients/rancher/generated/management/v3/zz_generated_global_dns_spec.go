package client

const (
	GlobalDNSSpecType                   = "globalDnsSpec"
	GlobalDNSSpecFieldFQDN              = "fqdn"
	GlobalDNSSpecFieldMembers           = "members"
	GlobalDNSSpecFieldMultiClusterAppID = "multiClusterAppId"
	GlobalDNSSpecFieldProjectIDs        = "projectIds"
	GlobalDNSSpecFieldProviderID        = "providerId"
	GlobalDNSSpecFieldTTL               = "ttl"
)

type GlobalDNSSpec struct {
	FQDN              string   `json:"fqdn,omitempty" yaml:"fqdn,omitempty"`
	Members           []Member `json:"members,omitempty" yaml:"members,omitempty"`
	MultiClusterAppID string   `json:"multiClusterAppId,omitempty" yaml:"multiClusterAppId,omitempty"`
	ProjectIDs        []string `json:"projectIds,omitempty" yaml:"projectIds,omitempty"`
	ProviderID        string   `json:"providerId,omitempty" yaml:"providerId,omitempty"`
	TTL               int64    `json:"ttl,omitempty" yaml:"ttl,omitempty"`
}
