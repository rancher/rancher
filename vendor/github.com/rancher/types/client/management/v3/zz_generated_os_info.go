package client

const (
	OSInfoType                 = "osInfo"
	OSInfoFieldDockerVersion   = "dockerVersion"
	OSInfoFieldKernelVersion   = "kernelVersion"
	OSInfoFieldOperatingSystem = "operatingSystem"
)

type OSInfo struct {
	DockerVersion   string `json:"dockerVersion,omitempty" yaml:"dockerVersion,omitempty"`
	KernelVersion   string `json:"kernelVersion,omitempty" yaml:"kernelVersion,omitempty"`
	OperatingSystem string `json:"operatingSystem,omitempty" yaml:"operatingSystem,omitempty"`
}
