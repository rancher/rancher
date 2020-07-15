package client

const (
	EnvVarType           = "envVar"
	EnvVarFieldName      = "name"
	EnvVarFieldValue     = "value"
	EnvVarFieldValueFrom = "valueFrom"
)

type EnvVar struct {
	Name      string        `json:"name,omitempty" yaml:"name,omitempty"`
	Value     string        `json:"value,omitempty" yaml:"value,omitempty"`
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty" yaml:"valueFrom,omitempty"`
}
