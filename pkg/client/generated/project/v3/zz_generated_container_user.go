package client

const (
	ContainerUserType       = "containerUser"
	ContainerUserFieldLinux = "linux"
)

type ContainerUser struct {
	Linux *LinuxContainerUser `json:"linux,omitempty" yaml:"linux,omitempty"`
}
