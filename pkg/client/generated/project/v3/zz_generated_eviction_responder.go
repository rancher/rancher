package client

const (
	EvictionResponderType          = "evictionResponder"
	EvictionResponderFieldName     = "name"
	EvictionResponderFieldPriority = "priority"
)

type EvictionResponder struct {
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Priority *int64 `json:"priority,omitempty" yaml:"priority,omitempty"`
}
