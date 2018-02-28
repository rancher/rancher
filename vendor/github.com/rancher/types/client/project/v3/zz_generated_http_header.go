package client

const (
	HTTPHeaderType       = "httpHeader"
	HTTPHeaderFieldName  = "name"
	HTTPHeaderFieldValue = "value"
)

type HTTPHeader struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
}
