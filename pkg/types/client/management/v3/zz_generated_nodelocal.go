package client

const (
	NodelocalType                = "nodelocal"
	NodelocalFieldIPAddress      = "ipAddress"
	NodelocalFieldNodeSelector   = "nodeSelector"
	NodelocalFieldUpdateStrategy = "updateStrategy"
)

type Nodelocal struct {
	IPAddress      string                   `json:"ipAddress,omitempty" yaml:"ipAddress,omitempty"`
	NodeSelector   map[string]string        `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	UpdateStrategy *DaemonSetUpdateStrategy `json:"updateStrategy,omitempty" yaml:"updateStrategy,omitempty"`
}
