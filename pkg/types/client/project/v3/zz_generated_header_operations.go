package client

const (
	HeaderOperationsType        = "headerOperations"
	HeaderOperationsFieldAdd    = "add"
	HeaderOperationsFieldRemove = "remove"
	HeaderOperationsFieldSet    = "set"
)

type HeaderOperations struct {
	Add    map[string]string `json:"add,omitempty" yaml:"add,omitempty"`
	Remove []string          `json:"remove,omitempty" yaml:"remove,omitempty"`
	Set    map[string]string `json:"set,omitempty" yaml:"set,omitempty"`
}
