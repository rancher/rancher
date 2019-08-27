package client

const (
	MonitoringConfigType          = "monitoringConfig"
	MonitoringConfigFieldOptions  = "options"
	MonitoringConfigFieldProvider = "provider"
)

type MonitoringConfig struct {
	Options  map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
	Provider string            `json:"provider,omitempty" yaml:"provider,omitempty"`
}
