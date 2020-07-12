package client

const (
	HeadersType          = "headers"
	HeadersFieldRequest  = "request"
	HeadersFieldResponse = "response"
)

type Headers struct {
	Request  *HeaderOperations `json:"request,omitempty" yaml:"request,omitempty"`
	Response *HeaderOperations `json:"response,omitempty" yaml:"response,omitempty"`
}
