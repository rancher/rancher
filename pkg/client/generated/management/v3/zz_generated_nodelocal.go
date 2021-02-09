package client

const (
	NodelocalType                               = "nodelocal"
	NodelocalFieldIPAddress                     = "ipAddress"
	NodelocalFieldNodeLocalDNSPriorityClassName = "nodeLocalDnsPriorityClassName"
	NodelocalFieldNodeSelector                  = "nodeSelector"
	NodelocalFieldUpdateStrategy                = "updateStrategy"
)

type Nodelocal struct {
	IPAddress                     string                   `json:"ipAddress,omitempty" yaml:"ipAddress,omitempty"`
	NodeLocalDNSPriorityClassName string                   `json:"nodeLocalDnsPriorityClassName,omitempty" yaml:"nodeLocalDnsPriorityClassName,omitempty"`
	NodeSelector                  map[string]string        `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	UpdateStrategy                *DaemonSetUpdateStrategy `json:"updateStrategy,omitempty" yaml:"updateStrategy,omitempty"`
}
