package client

const (
	ConfigMapProjectionType          = "configMapProjection"
	ConfigMapProjectionFieldItems    = "items"
	ConfigMapProjectionFieldName     = "name"
	ConfigMapProjectionFieldOptional = "optional"
)

type ConfigMapProjection struct {
	Items    []KeyToPath `json:"items,omitempty" yaml:"items,omitempty"`
	Name     string      `json:"name,omitempty" yaml:"name,omitempty"`
	Optional *bool       `json:"optional,omitempty" yaml:"optional,omitempty"`
}
