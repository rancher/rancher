package client

const (
	ContainerPortType               = "containerPort"
	ContainerPortFieldContainerPort = "containerPort"
	ContainerPortFieldHostIP        = "hostIP"
	ContainerPortFieldHostPort      = "hostPort"
	ContainerPortFieldProtocol      = "protocol"
)

type ContainerPort struct {
	ContainerPort *int64 `json:"containerPort,omitempty"`
	HostIP        string `json:"hostIP,omitempty"`
	HostPort      *int64 `json:"hostPort,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}
