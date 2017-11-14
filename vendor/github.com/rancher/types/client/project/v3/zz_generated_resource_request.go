package client

const (
	ResourceRequestType         = "resourceRequest"
	ResourceRequestFieldLimit   = "limit"
	ResourceRequestFieldRequest = "request"
)

type ResourceRequest struct {
	Limit   string `json:"limit,omitempty"`
	Request string `json:"request,omitempty"`
}
