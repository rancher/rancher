package client

const (
	IngressConfigType                                         = "ingressConfig"
	IngressConfigFieldDNSPolicy                               = "dnsPolicy"
	IngressConfigFieldDefaultBackend                          = "defaultBackend"
	IngressConfigFieldDefaultHTTPBackendPriorityClassName     = "defaultHttpBackendPriorityClassName"
	IngressConfigFieldDefaultIngressClass                     = "defaultIngressClass"
	IngressConfigFieldExtraArgs                               = "extraArgs"
	IngressConfigFieldExtraEnvs                               = "extraEnvs"
	IngressConfigFieldExtraVolumeMounts                       = "extraVolumeMounts"
	IngressConfigFieldExtraVolumes                            = "extraVolumes"
	IngressConfigFieldHTTPPort                                = "httpPort"
	IngressConfigFieldHTTPSPort                               = "httpsPort"
	IngressConfigFieldNetworkMode                             = "networkMode"
	IngressConfigFieldNginxIngressControllerPriorityClassName = "nginxIngressControllerPriorityClassName"
	IngressConfigFieldNodeSelector                            = "nodeSelector"
	IngressConfigFieldOptions                                 = "options"
	IngressConfigFieldProvider                                = "provider"
	IngressConfigFieldTolerations                             = "tolerations"
	IngressConfigFieldUpdateStrategy                          = "updateStrategy"
)

type IngressConfig struct {
	DNSPolicy                               string                   `json:"dnsPolicy,omitempty" yaml:"dnsPolicy,omitempty"`
	DefaultBackend                          *bool                    `json:"defaultBackend,omitempty" yaml:"defaultBackend,omitempty"`
	DefaultHTTPBackendPriorityClassName     string                   `json:"defaultHttpBackendPriorityClassName,omitempty" yaml:"defaultHttpBackendPriorityClassName,omitempty"`
	DefaultIngressClass                     *bool                    `json:"defaultIngressClass,omitempty" yaml:"defaultIngressClass,omitempty"`
	ExtraArgs                               map[string]string        `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ExtraEnvs                               []interface{}            `json:"extraEnvs,omitempty" yaml:"extraEnvs,omitempty"`
	ExtraVolumeMounts                       []interface{}            `json:"extraVolumeMounts,omitempty" yaml:"extraVolumeMounts,omitempty"`
	ExtraVolumes                            []interface{}            `json:"extraVolumes,omitempty" yaml:"extraVolumes,omitempty"`
	HTTPPort                                int64                    `json:"httpPort,omitempty" yaml:"httpPort,omitempty"`
	HTTPSPort                               int64                    `json:"httpsPort,omitempty" yaml:"httpsPort,omitempty"`
	NetworkMode                             string                   `json:"networkMode,omitempty" yaml:"networkMode,omitempty"`
	NginxIngressControllerPriorityClassName string                   `json:"nginxIngressControllerPriorityClassName,omitempty" yaml:"nginxIngressControllerPriorityClassName,omitempty"`
	NodeSelector                            map[string]string        `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Options                                 map[string]string        `json:"options,omitempty" yaml:"options,omitempty"`
	Provider                                string                   `json:"provider,omitempty" yaml:"provider,omitempty"`
	Tolerations                             []Toleration             `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	UpdateStrategy                          *DaemonSetUpdateStrategy `json:"updateStrategy,omitempty" yaml:"updateStrategy,omitempty"`
}
