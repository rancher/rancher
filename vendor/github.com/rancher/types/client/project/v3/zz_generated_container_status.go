package client

const (
	ContainerStatusType                      = "containerStatus"
	ContainerStatusFieldContainerID          = "containerID"
	ContainerStatusFieldImage                = "image"
	ContainerStatusFieldImageID              = "imageID"
	ContainerStatusFieldLastTerminationState = "lastState"
	ContainerStatusFieldName                 = "name"
	ContainerStatusFieldReady                = "ready"
	ContainerStatusFieldRestartCount         = "restartCount"
	ContainerStatusFieldStarted              = "started"
	ContainerStatusFieldState                = "state"
)

type ContainerStatus struct {
	ContainerID          string          `json:"containerID,omitempty" yaml:"containerID,omitempty"`
	Image                string          `json:"image,omitempty" yaml:"image,omitempty"`
	ImageID              string          `json:"imageID,omitempty" yaml:"imageID,omitempty"`
	LastTerminationState *ContainerState `json:"lastState,omitempty" yaml:"lastState,omitempty"`
	Name                 string          `json:"name,omitempty" yaml:"name,omitempty"`
	Ready                bool            `json:"ready,omitempty" yaml:"ready,omitempty"`
	RestartCount         int64           `json:"restartCount,omitempty" yaml:"restartCount,omitempty"`
	Started              *bool           `json:"started,omitempty" yaml:"started,omitempty"`
	State                *ContainerState `json:"state,omitempty" yaml:"state,omitempty"`
}
