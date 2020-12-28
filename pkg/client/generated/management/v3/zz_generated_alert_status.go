package client

const (
	AlertStatusType            = "alertStatus"
	AlertStatusFieldAlertState = "alertState"
)

type AlertStatus struct {
	AlertState string `json:"alertState,omitempty" yaml:"alertState,omitempty"`
}
