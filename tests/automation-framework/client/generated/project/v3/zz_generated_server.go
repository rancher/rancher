package client

const (
	ServerType       = "server"
	ServerFieldHosts = "hosts"
	ServerFieldPort  = "port"
	ServerFieldTLS   = "tls"
)

type Server struct {
	Hosts []string    `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	Port  *Port       `json:"port,omitempty" yaml:"port,omitempty"`
	TLS   *TLSOptions `json:"tls,omitempty" yaml:"tls,omitempty"`
}
