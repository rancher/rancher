package client

const (
	LabelSelectorRequirementType          = "labelSelectorRequirement"
	LabelSelectorRequirementFieldKey      = "key"
	LabelSelectorRequirementFieldOperator = "operator"
	LabelSelectorRequirementFieldValues   = "values"
)

type LabelSelectorRequirement struct {
	Key      string   `json:"key,omitempty"`
	Operator string   `json:"operator,omitempty"`
	Values   []string `json:"values,omitempty"`
}
