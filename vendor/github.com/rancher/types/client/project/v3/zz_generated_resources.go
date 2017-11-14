package client

const (
	ResourcesType           = "resources"
	ResourcesFieldCPU       = "cpu"
	ResourcesFieldMemory    = "memory"
	ResourcesFieldNvidiaGPU = "nvidiaGPU"
)

type Resources struct {
	CPU       *ResourceRequest `json:"cpu,omitempty"`
	Memory    *ResourceRequest `json:"memory,omitempty"`
	NvidiaGPU *ResourceRequest `json:"nvidiaGPU,omitempty"`
}
