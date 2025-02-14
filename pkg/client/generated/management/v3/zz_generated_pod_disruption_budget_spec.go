package client

const (
	PodDisruptionBudgetSpecType                = "podDisruptionBudgetSpec"
	PodDisruptionBudgetSpecFieldMaxUnavailable = "maxUnavailable"
	PodDisruptionBudgetSpecFieldMinAvailable   = "minAvailable"
)

type PodDisruptionBudgetSpec struct {
	MaxUnavailable string `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
	MinAvailable   string `json:"minAvailable,omitempty" yaml:"minAvailable,omitempty"`
}
