package client

const (
	IngressConfigType                   = "ingressConfig"
	IngressConfigFieldDNSPolicy         = "dnsPolicy"
	IngressConfigFieldExtraArgs         = "extraArgs"
	IngressConfigFieldExtraEnvs         = "extraEnvs"
	IngressConfigFieldExtraVolumeMounts = "extraVolumeMounts"
	IngressConfigFieldExtraVolumes      = "extraVolumes"
	IngressConfigFieldNodeSelector      = "nodeSelector"
	IngressConfigFieldOptions           = "options"
	IngressConfigFieldProvider          = "provider"
	IngressConfigFieldUpdateStrategy    = "updateStrategy"
)

type IngressConfig struct {
	DNSPolicy         string                   `json:"dnsPolicy,omitempty" yaml:"dnsPolicy,omitempty"`
	ExtraArgs         map[string]string        `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ExtraEnvs         []interface{}            `json:"extraEnvs,omitempty" yaml:"extraEnvs,omitempty"`
	ExtraVolumeMounts []interface{}            `json:"extraVolumeMounts,omitempty" yaml:"extraVolumeMounts,omitempty"`
	ExtraVolumes      []interface{}            `json:"extraVolumes,omitempty" yaml:"extraVolumes,omitempty"`
	NodeSelector      map[string]string        `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Options           map[string]string        `json:"options,omitempty" yaml:"options,omitempty"`
	Provider          string                   `json:"provider,omitempty" yaml:"provider,omitempty"`
	UpdateStrategy    *DaemonSetUpdateStrategy `json:"updateStrategy,omitempty" yaml:"updateStrategy,omitempty"`
}
