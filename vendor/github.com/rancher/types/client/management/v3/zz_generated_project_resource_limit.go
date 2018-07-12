package client

const (
	ProjectResourceLimitType                        = "projectResourceLimit"
	ProjectResourceLimitFieldConfigMaps             = "configMaps"
	ProjectResourceLimitFieldLimitsCPU              = "limitsCpu"
	ProjectResourceLimitFieldLimitsMemory           = "limitsMemory"
	ProjectResourceLimitFieldPersistentVolumeClaims = "persistentVolumeClaims"
	ProjectResourceLimitFieldPods                   = "pods"
	ProjectResourceLimitFieldReplicationControllers = "replicationControllers"
	ProjectResourceLimitFieldRequestsCPU            = "requestsCpu"
	ProjectResourceLimitFieldRequestsMemory         = "requestsMemory"
	ProjectResourceLimitFieldRequestsStorage        = "requestsStorage"
	ProjectResourceLimitFieldSecrets                = "secrets"
	ProjectResourceLimitFieldServices               = "services"
	ProjectResourceLimitFieldServicesLoadBalancers  = "servicesLoadBalancers"
	ProjectResourceLimitFieldServicesNodePorts      = "servicesNodePorts"
)

type ProjectResourceLimit struct {
	ConfigMaps             string `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
	LimitsCPU              string `json:"limitsCpu,omitempty" yaml:"limitsCpu,omitempty"`
	LimitsMemory           string `json:"limitsMemory,omitempty" yaml:"limitsMemory,omitempty"`
	PersistentVolumeClaims string `json:"persistentVolumeClaims,omitempty" yaml:"persistentVolumeClaims,omitempty"`
	Pods                   string `json:"pods,omitempty" yaml:"pods,omitempty"`
	ReplicationControllers string `json:"replicationControllers,omitempty" yaml:"replicationControllers,omitempty"`
	RequestsCPU            string `json:"requestsCpu,omitempty" yaml:"requestsCpu,omitempty"`
	RequestsMemory         string `json:"requestsMemory,omitempty" yaml:"requestsMemory,omitempty"`
	RequestsStorage        string `json:"requestsStorage,omitempty" yaml:"requestsStorage,omitempty"`
	Secrets                string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Services               string `json:"services,omitempty" yaml:"services,omitempty"`
	ServicesLoadBalancers  string `json:"servicesLoadBalancers,omitempty" yaml:"servicesLoadBalancers,omitempty"`
	ServicesNodePorts      string `json:"servicesNodePorts,omitempty" yaml:"servicesNodePorts,omitempty"`
}
