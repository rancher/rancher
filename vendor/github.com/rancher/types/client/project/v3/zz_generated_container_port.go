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
	ContainerPort int64  `json:"containerPort,omitempty" yaml:"containerPort,omitempty"`
	DNSName       string `json:"dnsName,omitempty" yaml:"dnsName,omitempty"`
	HostIp        string `json:"hostIp,omitempty" yaml:"hostIp,omitempty"`
	Kind          string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
	Protocol      string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	SourcePort    int64  `json:"sourcePort,omitempty" yaml:"sourcePort,omitempty"`
}
