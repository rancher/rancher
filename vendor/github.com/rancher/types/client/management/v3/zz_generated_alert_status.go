package client

const (
	AlertStatusType       = "alertStatus"
	AlertStatusFieldState = "state"
)

type AlertStatus struct {
	State string `json:"state,omitempty"`
}
