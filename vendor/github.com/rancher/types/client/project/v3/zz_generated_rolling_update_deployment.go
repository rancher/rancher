package client

const (
	RollingUpdateDeploymentType                = "rollingUpdateDeployment"
	RollingUpdateDeploymentFieldMaxSurge       = "maxSurge"
	RollingUpdateDeploymentFieldMaxUnavailable = "maxUnavailable"
)

type RollingUpdateDeployment struct {
	MaxSurge       string `json:"maxSurge,omitempty"`
	MaxUnavailable string `json:"maxUnavailable,omitempty"`
}
