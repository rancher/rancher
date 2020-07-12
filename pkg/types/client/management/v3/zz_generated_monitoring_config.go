package client

const (
	MonitoringConfigType                = "monitoringConfig"
	MonitoringConfigFieldNodeSelector   = "nodeSelector"
	MonitoringConfigFieldOptions        = "options"
	MonitoringConfigFieldProvider       = "provider"
	MonitoringConfigFieldReplicas       = "replicas"
	MonitoringConfigFieldUpdateStrategy = "updateStrategy"
)

type MonitoringConfig struct {
	NodeSelector   map[string]string   `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Options        map[string]string   `json:"options,omitempty" yaml:"options,omitempty"`
	Provider       string              `json:"provider,omitempty" yaml:"provider,omitempty"`
	Replicas       *int64              `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	UpdateStrategy *DeploymentStrategy `json:"updateStrategy,omitempty" yaml:"updateStrategy,omitempty"`
}
