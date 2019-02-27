package client

const (
	TCPSettingsType                = "tcpSettings"
	TCPSettingsFieldConnectTimeout = "connectTimeout"
	TCPSettingsFieldMaxConnections = "maxConnections"
)

type TCPSettings struct {
	ConnectTimeout string `json:"connectTimeout,omitempty" yaml:"connectTimeout,omitempty"`
	MaxConnections int64  `json:"maxConnections,omitempty" yaml:"maxConnections,omitempty"`
}
