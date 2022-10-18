package schema

type NodeInfo struct {
	CPU        CPUInfo
	Memory     MemoryInfo
	OS         OSInfo
	Kubernetes KubernetesInfo
}

type CPUInfo struct {
	Count int64
}

type MemoryInfo struct {
	MemTotalKiB int64
}

type OSInfo struct {
	DockerVersion   string
	KernelVersion   string
	OperatingSystem string
}

type KubernetesInfo struct {
	KubeletVersion   string
	KubeProxyVersion string
}
