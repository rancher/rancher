package client

const (
	TopologySelectorLabelRequirementType        = "topologySelectorLabelRequirement"
	TopologySelectorLabelRequirementFieldKey    = "key"
	TopologySelectorLabelRequirementFieldValues = "values"
)

type TopologySelectorLabelRequirement struct {
	Key    string   `json:"key,omitempty" yaml:"key,omitempty"`
	Values []string `json:"values,omitempty" yaml:"values,omitempty"`
}
