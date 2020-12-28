package client

const (
	ConnectionPoolSettingsType      = "connectionPoolSettings"
	ConnectionPoolSettingsFieldHTTP = "http"
	ConnectionPoolSettingsFieldTCP  = "tcp"
)

type ConnectionPoolSettings struct {
	HTTP *HTTPSettings `json:"http,omitempty" yaml:"http,omitempty"`
	TCP  *TCPSettings  `json:"tcp,omitempty" yaml:"tcp,omitempty"`
}
