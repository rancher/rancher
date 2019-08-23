package client

const (
	IngressConfigType              = "ingressConfig"
	IngressConfigFieldDNSPolicy    = "dnsPolicy"
	IngressConfigFieldExtraArgs    = "extraArgs"
	IngressConfigFieldNodeSelector = "nodeSelector"
	IngressConfigFieldOptions      = "options"
	IngressConfigFieldProvider     = "provider"
)

type IngressConfig struct {
	DNSPolicy    string            `json:"dnsPolicy,omitempty" yaml:"dnsPolicy,omitempty"`
	ExtraArgs    map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Options      map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
	Provider     string            `json:"provider,omitempty" yaml:"provider,omitempty"`
}
