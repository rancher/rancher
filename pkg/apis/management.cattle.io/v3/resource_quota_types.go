package v3

// ProjectResourceQuota represents the allowed and allocated quotas for all namespaces in a project.
type ProjectResourceQuota struct {
	// Limit is the total allowable quota limits shared by all namespaces in the project.
	// +optional
	Limit ResourceQuotaLimit `json:"limit,omitempty"`

	// UsedLimit is the currently allocated quota for all namespaces in the project.
	// +optional
	UsedLimit ResourceQuotaLimit `json:"usedLimit,omitempty"`
}

// NamespaceResourceQuota represents the default quota limits for a namespace.
type NamespaceResourceQuota struct {
	// Limit is the default quota limits applied to new namespaces.
	// +optional
	Limit ResourceQuotaLimit `json:"limit,omitempty"`
}

// ResourceQuotaLimit holds quota values for different resources.
// These resources are a subset of Kubernetes resources that can be limited.
// See https://kubernetes.io/docs/concepts/policy/resource-quotas/ for more details.
type ResourceQuotaLimit struct {
	// Pods is the total number of Pods in a non-terminal state that can exist in the namespace. A pod is in a terminal state if .status.phase in (Failed, Succeeded) is true.
	// +optional
	Pods string `json:"pods,omitempty"`

	// Services is the total number of Services that can exist in the namespace.
	// +optional
	Services string `json:"services,omitempty"`

	// ReplicationControllers is total number of ReplicationControllers that can exist in the namespace.
	// +optional
	ReplicationControllers string `json:"replicationControllers,omitempty"`

	// Secrets is the total number of ReplicationControllers that can exist in the namespace.
	// +optional
	Secrets string `json:"secrets,omitempty"`

	// ConfigMaps is the total number of ReplicationControllers that can exist in the namespace.
	// +optional
	ConfigMaps string `json:"configMaps,omitempty"`

	// PersistentVolumeClaims is the total number of PersistentVolumeClaims that can exist in the namespace.
	//  +optional
	PersistentVolumeClaims string `json:"persistentVolumeClaims,omitempty"`

	// ServiceNodePorts is the total number of Services of type NodePort that can exist in the namespace.
	// +optional
	ServicesNodePorts string `json:"servicesNodePorts,omitempty"`

	// ServicesLoadBalancers is the total number of Services of type LoadBalancer that can exist in the namespace.
	// +optional
	ServicesLoadBalancers string `json:"servicesLoadBalancers,omitempty"`

	// RequestsCPU is the CPU requests limit across all pods in a non-terminal state.
	// +optional
	RequestsCPU string `json:"requestsCpu,omitempty"`

	// RequestsMemory is the memory requests limit across all pods in a non-terminal state.
	// +optional
	RequestsMemory string `json:"requestsMemory,omitempty"`

	// RequestsStorage is the storage requests limit across all persistent volume claims.
	// +optional
	RequestsStorage string `json:"requestsStorage,omitempty"`

	// LimitsCPU is the CPU limits across all pods in a non-terminal state.
	// +optional
	LimitsCPU string `json:"limitsCpu,omitempty"`

	// LimitsMemory is the memory limits across all pods in a non-terminal state.
	// +optional
	LimitsMemory string `json:"limitsMemory,omitempty"`
}

// ContainerResourceLimit holds quotas limits for individual containers.
// These resources are a subset of Kubernetes resources that can be limited.
// See https://kubernetes.io/docs/concepts/policy/limit-range/ for more details.
type ContainerResourceLimit struct {
	// RequestsCPU is the CPU requests limit across all pods in a non-terminal state.
	// +optional
	RequestsCPU string `json:"requestsCpu,omitempty"`

	// RequestsMemory is the memory requests limit across all pods in a non-terminal state.
	// +optional
	RequestsMemory string `json:"requestsMemory,omitempty"`

	// LimitsCPU is the CPU limits across all pods in a non-terminal state.
	// +optional
	LimitsCPU string `json:"limitsCpu,omitempty"`

	// LimitsMemory is the memory limits across all pods in a non-terminal state.
	// +optional
	LimitsMemory string `json:"limitsMemory,omitempty"`
}
