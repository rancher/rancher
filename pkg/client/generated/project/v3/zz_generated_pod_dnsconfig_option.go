package client

const (
	PodDNSConfigOptionType       = "podDNSConfigOption"
	PodDNSConfigOptionFieldName  = "name"
	PodDNSConfigOptionFieldValue = "value"
)

type PodDNSConfigOption struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
}
