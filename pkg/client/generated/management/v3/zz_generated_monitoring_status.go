package client

const (
	MonitoringStatusType                 = "monitoringStatus"
	MonitoringStatusFieldConditions      = "conditions"
	MonitoringStatusFieldGrafanaEndpoint = "grafanaEndpoint"
)

type MonitoringStatus struct {
	Conditions      []MonitoringCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	GrafanaEndpoint string                `json:"grafanaEndpoint,omitempty" yaml:"grafanaEndpoint,omitempty"`
}
