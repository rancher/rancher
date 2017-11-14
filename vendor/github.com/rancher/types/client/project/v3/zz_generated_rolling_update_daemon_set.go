package client

const (
	RollingUpdateDaemonSetType                = "rollingUpdateDaemonSet"
	RollingUpdateDaemonSetFieldMaxUnavailable = "maxUnavailable"
)

type RollingUpdateDaemonSet struct {
	MaxUnavailable string `json:"maxUnavailable,omitempty"`
}
