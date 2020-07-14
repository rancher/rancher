package client

const (
	LabelSelectorRequirementType          = "labelSelectorRequirement"
	LabelSelectorRequirementFieldKey      = "key"
	LabelSelectorRequirementFieldOperator = "operator"
	LabelSelectorRequirementFieldValues   = "values"
)

type LabelSelectorRequirement struct {
	Key      string   `json:"key,omitempty" yaml:"key,omitempty"`
	Operator string   `json:"operator,omitempty" yaml:"operator,omitempty"`
	Values   []string `json:"values,omitempty" yaml:"values,omitempty"`
}
