package client

const (
	OSInfoType                 = "osInfo"
	OSInfoFieldDockerVersion   = "dockerVersion"
	OSInfoFieldKernelVersion   = "kernelVersion"
	OSInfoFieldOperatingSystem = "operatingSystem"
)

type OSInfo struct {
	DockerVersion   string `json:"dockerVersion,omitempty"`
	KernelVersion   string `json:"kernelVersion,omitempty"`
	OperatingSystem string `json:"operatingSystem,omitempty"`
}
