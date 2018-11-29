package client

const (
	AlertingSpecType               = "alertingSpec"
	AlertingSpecFieldAlertmanagers = "alertmanagers"
)

type AlertingSpec struct {
	Alertmanagers []AlertmanagerEndpoints `json:"alertmanagers,omitempty" yaml:"alertmanagers,omitempty"`
}
