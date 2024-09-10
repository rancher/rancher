package client

const (
	ContainerResizePolicyType               = "containerResizePolicy"
	ContainerResizePolicyFieldResourceName  = "resourceName"
	ContainerResizePolicyFieldRestartPolicy = "restartPolicy"
)

type ContainerResizePolicy struct {
	ResourceName  string `json:"resourceName,omitempty" yaml:"resourceName,omitempty"`
	RestartPolicy string `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
}
