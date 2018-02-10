package client

const (
	InitializersType         = "initializers"
	InitializersFieldPending = "pending"
	InitializersFieldResult  = "result"
)

type Initializers struct {
	Pending []Initializer `json:"pending,omitempty"`
	Result  *Status       `json:"result,omitempty"`
}
