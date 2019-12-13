package client

const (
	NodelocalType           = "nodelocal"
	NodelocalFieldIPAddress = "ipAddress"
)

type Nodelocal struct {
	IPAddress string `json:"ipAddress,omitempty" yaml:"ipAddress,omitempty"`
}
