package client

const (
	RKESystemConfigType                      = "rkeSystemConfig"
	RKESystemConfigFieldConfig               = "config"
	RKESystemConfigFieldMachineLabelSelector = "machineLabelSelector"
)

type RKESystemConfig struct {
	Config               *GenericMap    `json:"config,omitempty" yaml:"config,omitempty"`
	MachineLabelSelector *LabelSelector `json:"machineLabelSelector,omitempty" yaml:"machineLabelSelector,omitempty"`
}
