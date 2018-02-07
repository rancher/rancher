package client

const (
	IngressConfigType              = "ingressConfig"
	IngressConfigFieldNodeSelector = "nodeSelector"
	IngressConfigFieldOptions      = "options"
	IngressConfigFieldProvider     = "provider"
)

type IngressConfig struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	Options      map[string]string `json:"options,omitempty"`
	Provider     string            `json:"provider,omitempty"`
}
