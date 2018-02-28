package client

const (
	InitializersType         = "initializers"
	InitializersFieldPending = "pending"
	InitializersFieldResult  = "result"
)

type Initializers struct {
	Pending []Initializer `json:"pending,omitempty" yaml:"pending,omitempty"`
	Result  *Status       `json:"result,omitempty" yaml:"result,omitempty"`
}
