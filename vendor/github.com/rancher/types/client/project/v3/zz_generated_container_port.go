package client

const (
	ContainerPortType               = "containerPort"
	ContainerPortFieldContainerPort = "containerPort"
	ContainerPortFieldDNSName       = "dnsName"
	ContainerPortFieldHostIp        = "hostIp"
	ContainerPortFieldKind          = "kind"
	ContainerPortFieldName          = "name"
	ContainerPortFieldProtocol      = "protocol"
	ContainerPortFieldSourcePort    = "sourcePort"
)

type ContainerPort struct {
	ContainerPort *int64 `json:"containerPort,omitempty"`
	DNSName       string `json:"dnsName,omitempty"`
	HostIp        string `json:"hostIp,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Name          string `json:"name,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	SourcePort    *int64 `json:"sourcePort,omitempty"`
}
