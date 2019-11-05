package client

const (
	ContainerPortType               = "containerPort"
	ContainerPortFieldContainerPort = "containerPort"
	ContainerPortFieldDNSName       = "dnsName"
	ContainerPortFieldHostIp        = "hostIp"
	ContainerPortFieldHostPort      = "sourcePort"
	ContainerPortFieldKind          = "kind"
	ContainerPortFieldName          = "name"
	ContainerPortFieldProtocol      = "protocol"
)

type ContainerPort struct {
	ContainerPort int64  `json:"containerPort,omitempty" yaml:"containerPort,omitempty"`
	DNSName       string `json:"dnsName,omitempty" yaml:"dnsName,omitempty"`
	HostIp        string `json:"hostIp,omitempty" yaml:"hostIp,omitempty"`
	HostPort      int64  `json:"sourcePort,omitempty" yaml:"sourcePort,omitempty"`
	Kind          string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
	Protocol      string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
}
