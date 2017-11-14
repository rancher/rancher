package client

const (
	NodeSelectorRequirementType          = "nodeSelectorRequirement"
	NodeSelectorRequirementFieldKey      = "key"
	NodeSelectorRequirementFieldOperator = "operator"
	NodeSelectorRequirementFieldValues   = "values"
)

type NodeSelectorRequirement struct {
	Key      string   `json:"key,omitempty"`
	Operator string   `json:"operator,omitempty"`
	Values   []string `json:"values,omitempty"`
}
