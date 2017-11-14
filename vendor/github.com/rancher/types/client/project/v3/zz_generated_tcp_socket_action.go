package client

const (
	TCPSocketActionType      = "tcpSocketAction"
	TCPSocketActionFieldHost = "host"
	TCPSocketActionFieldPort = "port"
)

type TCPSocketAction struct {
	Host string `json:"host,omitempty"`
	Port string `json:"port,omitempty"`
}
