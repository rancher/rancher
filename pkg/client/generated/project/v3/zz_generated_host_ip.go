package client

const (
	HostIPType    = "hostIP"
	HostIPFieldIP = "ip"
)

type HostIP struct {
	IP string `json:"ip,omitempty" yaml:"ip,omitempty"`
}
