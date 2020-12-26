package client

const (
	EnvironmentVarType           = "environmentVar"
	EnvironmentVarFieldName      = "name"
	EnvironmentVarFieldValue     = "value"
	EnvironmentVarFieldValueFrom = "valueFrom"
)

type EnvironmentVar struct {
	Name      string           `json:"name,omitempty" yaml:"name,omitempty"`
	Value     string           `json:"value,omitempty" yaml:"value,omitempty"`
	ValueFrom *EnvironmentFrom `json:"valueFrom,omitempty" yaml:"valueFrom,omitempty"`
}
