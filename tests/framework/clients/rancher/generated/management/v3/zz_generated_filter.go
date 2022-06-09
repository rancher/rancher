package client

const (
	FilterType           = "filter"
	FilterFieldModifiers = "modifiers"
)

type Filter struct {
	Modifiers []string `json:"modifiers,omitempty" yaml:"modifiers,omitempty"`
}
