package client

const (
	InitializerType      = "initializer"
	InitializerFieldName = "name"
)

type Initializer struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}
