package client

const (
	IngressConfigType              = "ingressConfig"
	IngressConfigFieldNodeSelector = "nodeSelector"
	IngressConfigFieldOptions      = "options"
	IngressConfigFieldType         = "type"
)

type IngressConfig struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	Options      map[string]string `json:"options,omitempty"`
	Type         string            `json:"type,omitempty"`
}
