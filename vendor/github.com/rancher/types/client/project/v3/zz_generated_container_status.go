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
	ContainerStatusFieldState                = "state"
)

type ContainerStatus struct {
	ContainerID          string          `json:"containerID,omitempty"`
	Image                string          `json:"image,omitempty"`
	ImageID              string          `json:"imageID,omitempty"`
	LastTerminationState *ContainerState `json:"lastState,omitempty"`
	Name                 string          `json:"name,omitempty"`
	Ready                bool            `json:"ready,omitempty"`
	RestartCount         *int64          `json:"restartCount,omitempty"`
	State                *ContainerState `json:"state,omitempty"`
}
