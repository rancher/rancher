package client

const (
	ServicePortType            = "servicePort"
	ServicePortFieldName       = "name"
	ServicePortFieldNodePort   = "nodePort"
	ServicePortFieldPort       = "port"
	ServicePortFieldProtocol   = "protocol"
	ServicePortFieldTargetPort = "targetPort"
)

type ServicePort struct {
	Name       string `json:"name,omitempty"`
	NodePort   *int64 `json:"nodePort,omitempty"`
	Port       *int64 `json:"port,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	TargetPort string `json:"targetPort,omitempty"`
}
