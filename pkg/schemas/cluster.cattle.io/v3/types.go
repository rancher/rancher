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

type NamespaceResourceQuota struct {
	Limit ResourceQuotaLimit `json:"limit,omitempty"`
}

type ResourceQuotaLimit struct {
	Pods                   string `json:"pods,omitempty"`
	Services               string `json:"services,omitempty"`
	ReplicationControllers string `json:"replicationControllers,omitempty"`
	Secrets                string `json:"secrets,omitempty"`
	ConfigMaps             string `json:"configMaps,omitempty"`
	PersistentVolumeClaims string `json:"persistentVolumeClaims,omitempty"`
	ServicesNodePorts      string `json:"servicesNodePorts,omitempty"`
	ServicesLoadBalancers  string `json:"servicesLoadBalancers,omitempty"`
	RequestsCPU            string `json:"requestsCpu,omitempty"`
	RequestsMemory         string `json:"requestsMemory,omitempty"`
	RequestsStorage        string `json:"requestsStorage,omitempty"`
	LimitsCPU              string `json:"limitsCpu,omitempty"`
	LimitsMemory           string `json:"limitsMemory,omitempty"`
}

type NamespaceMove struct {
	ProjectID string `json:"projectId,omitempty"`
}

type ContainerResourceLimit struct {
	RequestsCPU    string `json:"requestsCpu,omitempty"`
	RequestsMemory string `json:"requestsMemory,omitempty"`
	LimitsCPU      string `json:"limitsCpu,omitempty"`
	LimitsMemory   string `json:"limitsMemory,omitempty"`
}
