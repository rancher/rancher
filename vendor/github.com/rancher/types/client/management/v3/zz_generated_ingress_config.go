package client

const (
	IngressConfigType                = "ingressConfig"
	IngressConfigFieldExtraArguments = "extraArguments"
	IngressConfigFieldNodeSelector   = "nodeSelector"
	IngressConfigFieldOptions        = "options"
	IngressConfigFieldProvider       = "provider"
)

type IngressConfig struct {
	ExtraArguments []string          `json:"extraArguments,omitempty" yaml:"extraArguments,omitempty"`
	NodeSelector   map[string]string `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Options        map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
	Provider       string            `json:"provider,omitempty" yaml:"provider,omitempty"`
}
