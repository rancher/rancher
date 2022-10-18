package client

const (
	DaemonEndpointType      = "daemonEndpoint"
	DaemonEndpointFieldPort = "Port"
)

type DaemonEndpoint struct {
	Port int64 `json:"Port,omitempty" yaml:"Port,omitempty"`
}
