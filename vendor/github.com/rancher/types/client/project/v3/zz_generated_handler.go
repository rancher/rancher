package client

const (
	HandlerType             = "handler"
	HandlerFieldCommand     = "command"
	HandlerFieldHTTPHeaders = "httpHeaders"
	HandlerFieldHost        = "host"
	HandlerFieldPath        = "path"
	HandlerFieldPort        = "port"
	HandlerFieldScheme      = "scheme"
	HandlerFieldTCP         = "tcp"
)

type Handler struct {
	Command     []string     `json:"command,omitempty"`
	HTTPHeaders []HTTPHeader `json:"httpHeaders,omitempty"`
	Host        string       `json:"host,omitempty"`
	Path        string       `json:"path,omitempty"`
	Port        string       `json:"port,omitempty"`
	Scheme      string       `json:"scheme,omitempty"`
	TCP         *bool        `json:"tcp,omitempty"`
}
