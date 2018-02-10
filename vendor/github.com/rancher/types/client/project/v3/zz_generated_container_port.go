package client

const (
	ContainerPortType               = "containerPort"
	ContainerPortFieldContainerPort = "containerPort"
	ContainerPortFieldHostIp        = "hostIp"
	ContainerPortFieldHostPort      = "hostPort"
	ContainerPortFieldName          = "name"
	ContainerPortFieldProtocol      = "protocol"
)

type ContainerPort struct {
	ContainerPort *int64 `json:"containerPort,omitempty"`
	HostIp        string `json:"hostIp,omitempty"`
	HostPort      *int64 `json:"hostPort,omitempty"`
	Name          string `json:"name,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}
