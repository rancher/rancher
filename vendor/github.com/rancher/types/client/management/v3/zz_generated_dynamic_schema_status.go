package client

const (
	DynamicSchemaStatusType      = "dynamicSchemaStatus"
	DynamicSchemaStatusFieldFake = "fake"
)

type DynamicSchemaStatus struct {
	Fake string `json:"fake,omitempty" yaml:"fake,omitempty"`
}
