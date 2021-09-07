package client

const (
	ContainerStateType            = "containerState"
	ContainerStateFieldRunning    = "running"
	ContainerStateFieldTerminated = "terminated"
	ContainerStateFieldWaiting    = "waiting"
)

type ContainerState struct {
	Running    *ContainerStateRunning    `json:"running,omitempty" yaml:"running,omitempty"`
	Terminated *ContainerStateTerminated `json:"terminated,omitempty" yaml:"terminated,omitempty"`
	Waiting    *ContainerStateWaiting    `json:"waiting,omitempty" yaml:"waiting,omitempty"`
}
