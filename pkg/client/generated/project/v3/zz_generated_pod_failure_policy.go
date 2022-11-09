package client

const (
	PodFailurePolicyType       = "podFailurePolicy"
	PodFailurePolicyFieldRules = "rules"
)

type PodFailurePolicy struct {
	Rules []PodFailurePolicyRule `json:"rules,omitempty" yaml:"rules,omitempty"`
}
