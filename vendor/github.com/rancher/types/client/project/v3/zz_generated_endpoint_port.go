package client

const (
	EndpointPortType          = "endpointPort"
	EndpointPortFieldName     = "name"
	EndpointPortFieldPort     = "port"
	EndpointPortFieldProtocol = "protocol"
)

type EndpointPort struct {
	Name     string `json:"name,omitempty"`
	Port     *int64 `json:"port,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}
