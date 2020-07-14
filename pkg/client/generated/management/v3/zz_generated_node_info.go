package client

const (
	NodeInfoType            = "nodeInfo"
	NodeInfoFieldCPU        = "cpu"
	NodeInfoFieldKubernetes = "kubernetes"
	NodeInfoFieldMemory     = "memory"
	NodeInfoFieldOS         = "os"
)

type NodeInfo struct {
	CPU        *CPUInfo        `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Kubernetes *KubernetesInfo `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	Memory     *MemoryInfo     `json:"memory,omitempty" yaml:"memory,omitempty"`
	OS         *OSInfo         `json:"os,omitempty" yaml:"os,omitempty"`
}
