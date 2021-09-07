package client

const (
	PodDNSConfigType             = "podDNSConfig"
	PodDNSConfigFieldNameservers = "nameservers"
	PodDNSConfigFieldOptions     = "options"
	PodDNSConfigFieldSearches    = "searches"
)

type PodDNSConfig struct {
	Nameservers []string             `json:"nameservers,omitempty" yaml:"nameservers,omitempty"`
	Options     []PodDNSConfigOption `json:"options,omitempty" yaml:"options,omitempty"`
	Searches    []string             `json:"searches,omitempty" yaml:"searches,omitempty"`
}
