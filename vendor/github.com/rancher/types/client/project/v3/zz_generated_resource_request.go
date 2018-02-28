package client

const (
	ResourceRequestType         = "resourceRequest"
	ResourceRequestFieldLimit   = "limit"
	ResourceRequestFieldRequest = "request"
)

type ResourceRequest struct {
	Limit   string `json:"limit,omitempty" yaml:"limit,omitempty"`
	Request string `json:"request,omitempty" yaml:"request,omitempty"`
}
