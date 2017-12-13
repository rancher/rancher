package client

const (
	ContainerPortType               = "containerPort"
	ContainerPortFieldContainerPort = "containerPort"
	ContainerPortFieldHostIp        = "hostIp"
	ContainerPortFieldHostPort      = "hostPort"
	ContainerPortFieldProtocol      = "protocol"
)

type ContainerPort struct {
	ContainerPort *int64 `json:"containerPort,omitempty"`
	HostIp        string `json:"hostIp,omitempty"`
	HostPort      *int64 `json:"hostPort,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}
