package v3

type RancherKubernetesEngineConfig struct {
	// Kubernetes nodes
	Nodes []RKEConfigNode `yaml:"nodes" json:"nodes,omitempty"`
	// Kubernetes components
	Services RKEConfigServices `yaml:"services" json:"services,omitempty"`
	// Network configuration used in the kubernetes cluster (flannel, calico)
	Network NetworkConfig `yaml:"network" json:"network,omitempty"`
	// Authentication configuration used in the cluster (default: x509)
	Authentication AuthConfig `yaml:"auth" json:"auth,omitempty"`
	// YAML manifest for user provided addons to be deployed on the cluster
	Addons string `yaml:"addons" json:"addons,omitempty"`
	// List of images used internally for proxy, cert downlaod and kubedns
	SystemImages map[string]string `yaml:"system_images" json:"systemImages,omitempty"`
	// SSH Private Key Path
	SSHKeyPath string `yaml:"ssh_key_path" json:"sshKeyPath,omitempty"`
}

type RKEConfigNode struct {
	// IP or FQDN that is fully resolvable and used for SSH communication
	Address string `yaml:"address" json:"address,omitempty"`
	// Optional - Internal address that will be used for components communication
	InternalAddress string `yaml:"internal_address" json:"internalAddress,omitempty"`
	// Node role in kubernetes cluster (controlplane, worker, or etcd)
	Role []string `yaml:"role" json:"role,omitempty"`
	// Optional - Hostname of the node
	HostnameOverride string `yaml:"hostname_override" json:"hostnameOverride,omitempty"`
	// SSH usesr that will be used by RKE
	User string `yaml:"user" json:"user,omitempty"`
	// Optional - Docker socket on the node that will be used in tunneling
	DockerSocket string `yaml:"docker_socket" json:"dockerSocket,omitempty"`
	// SSH Private Key
	SSHKey string `yaml:"ssh_key" json:"sshKey,omitempty"`
	// SSH Private Key Path
	SSHKeyPath string `yaml:"ssh_key_path" json:"sshKeyPath,omitempty"`
}

type RKEConfigServices struct {
	// Etcd Service
	Etcd ETCDService `yaml:"etcd" json:"etcd,omitempty"`
	// KubeAPI Service
	KubeAPI KubeAPIService `yaml:"kube-api" json:"kubeApi,omitempty"`
	// KubeController Service
	KubeController KubeControllerService `yaml:"kube-controller" json:"kubeController,omitempty"`
	// Scheduler Service
	Scheduler SchedulerService `yaml:"scheduler" json:"scheduler,omitempty"`
	// Kubelet Service
	Kubelet KubeletService `yaml:"kubelet" json:"kubelet,omitempty"`
	// KubeProxy Service
	Kubeproxy KubeproxyService `yaml:"kubeproxy" json:"kubeproxy,omitempty"`
}

type ETCDService struct {
	// Base service properties
	BaseService `yaml:",inline" json:",inline"`
}

type KubeAPIService struct {
	// Base service properties
	BaseService `yaml:",inline" json:",inline"`
	// Virtual IP range that will be used by Kubernetes services
	ServiceClusterIPRange string `yaml:"service_cluster_ip_range" json:"serviceClusterIpRange,omitempty"`
}

type KubeControllerService struct {
	// Base service properties
	BaseService `yaml:",inline" json:",inline"`
	// CIDR Range for Pods in cluster
	ClusterCIDR string `yaml:"cluster_cidr" json:"clusterCidr,omitempty"`
	// Virtual IP range that will be used by Kubernetes services
	ServiceClusterIPRange string `yaml:"service_cluster_ip_range" json:"serviceClusterIpRange,omitempty"`
}

type KubeletService struct {
	// Base service properties
	BaseService `yaml:",inline" json:",inline"`
	// Domain of the cluster (default: "cluster.local")
	ClusterDomain string `yaml:"cluster_domain" json:"clusterDomain,omitempty"`
	// The image whose network/ipc namespaces containers in each pod will use
	InfraContainerImage string `yaml:"infra_container_image" json:"infraContainerImage,omitempty"`
	// Cluster DNS service ip
	ClusterDNSServer string `yaml:"cluster_dns_server" json:"clusterDnsServer,omitempty"`
}

type KubeproxyService struct {
	// Base service properties
	BaseService `yaml:",inline" json:",inline"`
}

type SchedulerService struct {
	// Base service properties
	BaseService `yaml:",inline" json:",inline"`
}

type BaseService struct {
	// Docker image of the service
	Image string `yaml:"image" json:"image,omitempty"`
	// Extra arguments that are added to the services
	ExtraArgs map[string]string `yaml:"extra_args" json:"extraArgs,omitempty"`
}

type NetworkConfig struct {
	// Network Plugin That will be used in kubernetes cluster
	Plugin string `yaml:"plugin" json:"plugin,omitempty"`
	// Plugin options to configure network properties
	Options map[string]string `yaml:"options" json:"options,omitempty"`
}

type AuthConfig struct {
	// Authentication strategy that will be used in kubernetes cluster
	Strategy string `yaml:"strategy" json:"strategy,omitempty"`
	// Authentication options
	Options map[string]string `yaml:"options" json:"options,omitempty"`
}
