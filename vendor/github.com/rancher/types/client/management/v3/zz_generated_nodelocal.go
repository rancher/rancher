package client

const (
	NodelocalType              = "nodelocal"
	NodelocalFieldIPAddress    = "ipAddress"
	NodelocalFieldNodeSelector = "nodeSelector"
)

type Nodelocal struct {
	IPAddress    string            `json:"ipAddress,omitempty" yaml:"ipAddress,omitempty"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
}
