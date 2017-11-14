package client

const (
	SecretProjectionType          = "secretProjection"
	SecretProjectionFieldItems    = "items"
	SecretProjectionFieldName     = "name"
	SecretProjectionFieldOptional = "optional"
)

type SecretProjection struct {
	Items    []KeyToPath `json:"items,omitempty"`
	Name     string      `json:"name,omitempty"`
	Optional *bool       `json:"optional,omitempty"`
}
