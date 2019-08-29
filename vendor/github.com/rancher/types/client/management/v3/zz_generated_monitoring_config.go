package client

const (
	MonitoringConfigType              = "monitoringConfig"
	MonitoringConfigFieldNodeSelector = "nodeSelector"
	MonitoringConfigFieldOptions      = "options"
	MonitoringConfigFieldProvider     = "provider"
)

type MonitoringConfig struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Options      map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
	Provider     string            `json:"provider,omitempty" yaml:"provider,omitempty"`
}
