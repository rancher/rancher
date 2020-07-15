package client

const (
	ExtraEnvType           = "extraEnv"
	ExtraEnvFieldName      = "name"
	ExtraEnvFieldValue     = "value"
	ExtraEnvFieldValueFrom = "valueFrom"
)

type ExtraEnv struct {
	Name      string        `json:"name,omitempty" yaml:"name,omitempty"`
	Value     string        `json:"value,omitempty" yaml:"value,omitempty"`
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty" yaml:"valueFrom,omitempty"`
}
