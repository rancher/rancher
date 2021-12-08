package client

const (
	GlobalOpenstackOptsType            = "globalOpenstackOpts"
	GlobalOpenstackOptsFieldAuthURL    = "auth-url"
	GlobalOpenstackOptsFieldCAFile     = "ca-file"
	GlobalOpenstackOptsFieldDomainID   = "domain-id"
	GlobalOpenstackOptsFieldDomainName = "domain-name"
	GlobalOpenstackOptsFieldPassword   = "password"
	GlobalOpenstackOptsFieldRegion     = "region"
	GlobalOpenstackOptsFieldTenantID   = "tenant-id"
	GlobalOpenstackOptsFieldTenantName = "tenant-name"
	GlobalOpenstackOptsFieldTrustID    = "trust-id"
	GlobalOpenstackOptsFieldUserID     = "user-id"
	GlobalOpenstackOptsFieldUsername   = "username"
)

type GlobalOpenstackOpts struct {
	AuthURL    string `json:"auth-url,omitempty" yaml:"auth-url,omitempty"`
	CAFile     string `json:"ca-file,omitempty" yaml:"ca-file,omitempty"`
	DomainID   string `json:"domain-id,omitempty" yaml:"domain-id,omitempty"`
	DomainName string `json:"domain-name,omitempty" yaml:"domain-name,omitempty"`
	Password   string `json:"password,omitempty" yaml:"password,omitempty"`
	Region     string `json:"region,omitempty" yaml:"region,omitempty"`
	TenantID   string `json:"tenant-id,omitempty" yaml:"tenant-id,omitempty"`
	TenantName string `json:"tenant-name,omitempty" yaml:"tenant-name,omitempty"`
	TrustID    string `json:"trust-id,omitempty" yaml:"trust-id,omitempty"`
	UserID     string `json:"user-id,omitempty" yaml:"user-id,omitempty"`
	Username   string `json:"username,omitempty" yaml:"username,omitempty"`
}
