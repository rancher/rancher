package client

const (
	NodeInfoType            = "nodeInfo"
	NodeInfoFieldCPU        = "cpu"
	NodeInfoFieldKubernetes = "kubernetes"
	NodeInfoFieldMemory     = "memory"
	NodeInfoFieldOS         = "os"
)

type NodeInfo struct {
	CPU        *CPUInfo        `json:"cpu,omitempty"`
	Kubernetes *KubernetesInfo `json:"kubernetes,omitempty"`
	Memory     *MemoryInfo     `json:"memory,omitempty"`
	OS         *OSInfo         `json:"os,omitempty"`
}
