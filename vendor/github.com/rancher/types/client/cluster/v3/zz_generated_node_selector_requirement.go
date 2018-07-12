package client

const (
	NodeSelectorRequirementType          = "nodeSelectorRequirement"
	NodeSelectorRequirementFieldKey      = "key"
	NodeSelectorRequirementFieldOperator = "operator"
	NodeSelectorRequirementFieldValues   = "values"
)

type NodeSelectorRequirement struct {
	Key      string   `json:"key,omitempty" yaml:"key,omitempty"`
	Operator string   `json:"operator,omitempty" yaml:"operator,omitempty"`
	Values   []string `json:"values,omitempty" yaml:"values,omitempty"`
}
