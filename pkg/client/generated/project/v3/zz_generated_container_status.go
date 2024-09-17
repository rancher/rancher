package client

const (
	ContainerStatusType                          = "containerStatus"
	ContainerStatusFieldAllocatedResources       = "allocatedResources"
	ContainerStatusFieldAllocatedResourcesStatus = "allocatedResourcesStatus"
	ContainerStatusFieldContainerID              = "containerID"
	ContainerStatusFieldImage                    = "image"
	ContainerStatusFieldImageID                  = "imageID"
	ContainerStatusFieldLastTerminationState     = "lastState"
	ContainerStatusFieldName                     = "name"
	ContainerStatusFieldReady                    = "ready"
	ContainerStatusFieldResources                = "resources"
	ContainerStatusFieldRestartCount             = "restartCount"
	ContainerStatusFieldStarted                  = "started"
	ContainerStatusFieldState                    = "state"
	ContainerStatusFieldUser                     = "user"
	ContainerStatusFieldVolumeMounts             = "volumeMounts"
)

type ContainerStatus struct {
	AllocatedResources       map[string]string     `json:"allocatedResources,omitempty" yaml:"allocatedResources,omitempty"`
	AllocatedResourcesStatus []ResourceStatus      `json:"allocatedResourcesStatus,omitempty" yaml:"allocatedResourcesStatus,omitempty"`
	ContainerID              string                `json:"containerID,omitempty" yaml:"containerID,omitempty"`
	Image                    string                `json:"image,omitempty" yaml:"image,omitempty"`
	ImageID                  string                `json:"imageID,omitempty" yaml:"imageID,omitempty"`
	LastTerminationState     *ContainerState       `json:"lastState,omitempty" yaml:"lastState,omitempty"`
	Name                     string                `json:"name,omitempty" yaml:"name,omitempty"`
	Ready                    bool                  `json:"ready,omitempty" yaml:"ready,omitempty"`
	Resources                *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	RestartCount             int64                 `json:"restartCount,omitempty" yaml:"restartCount,omitempty"`
	Started                  *bool                 `json:"started,omitempty" yaml:"started,omitempty"`
	State                    *ContainerState       `json:"state,omitempty" yaml:"state,omitempty"`
	User                     *ContainerUser        `json:"user,omitempty" yaml:"user,omitempty"`
	VolumeMounts             []VolumeMountStatus   `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
}
