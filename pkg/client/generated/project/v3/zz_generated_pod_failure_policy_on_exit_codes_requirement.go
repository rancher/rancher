package client

const (
	PodFailurePolicyOnExitCodesRequirementType               = "podFailurePolicyOnExitCodesRequirement"
	PodFailurePolicyOnExitCodesRequirementFieldContainerName = "containerName"
	PodFailurePolicyOnExitCodesRequirementFieldOperator      = "operator"
	PodFailurePolicyOnExitCodesRequirementFieldValues        = "values"
)

type PodFailurePolicyOnExitCodesRequirement struct {
	ContainerName string  `json:"containerName,omitempty" yaml:"containerName,omitempty"`
	Operator      string  `json:"operator,omitempty" yaml:"operator,omitempty"`
	Values        []int64 `json:"values,omitempty" yaml:"values,omitempty"`
}
