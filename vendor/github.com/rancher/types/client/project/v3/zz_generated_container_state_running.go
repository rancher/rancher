package client

const (
	ContainerStateRunningType           = "containerStateRunning"
	ContainerStateRunningFieldStartedAt = "startedAt"
)

type ContainerStateRunning struct {
	StartedAt string `json:"startedAt,omitempty"`
}
