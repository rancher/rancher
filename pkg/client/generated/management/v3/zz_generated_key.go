package client

const (
	KeyType        = "key"
	KeyFieldName   = "name"
	KeyFieldSecret = "secret"
)

type Key struct {
	Name   string `json:"name,omitempty" yaml:"name,omitempty"`
	Secret string `json:"secret,omitempty" yaml:"secret,omitempty"`
}
