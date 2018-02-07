package v3

type RancherKubernetesEngineConfig struct {
	// Kubernetes nodes
	Nodes []RKEConfigNode `yaml:"nodes" json:"nodes,omitempty"`
	// Kubernetes components
	Services RKEConfigServices `yaml:"services" json:"services,omitempty"`
	// Network configuration used in the kubernetes cluster (flannel, calico)
	Network NetworkConfig `yaml:"network" json:"network,omitempty"`
	// Authentication configuration used in the cluster (default: x509)
	Authentication AuthnConfig `yaml:"authentication" json:"authentication,omitempty"`
	// YAML manifest for user provided addons to be deployed on the cluster
	Addons string `yaml:"addons" json:"addons,omitempty"`
	// List of images used internally for proxy, cert downlaod and kubedns
	SystemImages RKESystemImages `yaml:"system_images" json:"systemImages,omitempty"`
	// SSH Private Key Path
	SSHKeyPath string `yaml:"ssh_key_path" json:"sshKeyPath,omitempty"`
	// Authorization mode configuration used in the cluster
	Authorization AuthzConfig `yaml:"authorization" json:"authorization,omitempty"`
	// Enable/disable strict docker version checking
	IgnoreDockerVersion bool `yaml:"ignore_docker_version" json:"ignoreDockerVersion"`
	// Kubernetes version to use (if kubernetes image is specifed, image version takes precedence)
	Version string `yaml:"kubernetes_version" json:"kubernetesVersion,omitempty"`
	// List of private registries and their credentials
	PrivateRegistries []PrivateRegistry `yaml:"private_registries" json:"privateRegistries,omitempty"`
	// Ingress controller used in the cluster
	Ingress IngressConfig `yaml:"ingress" json:"ingress,omitempty"`
}

type PrivateRegistry struct {
	// URL for the registry
	URL string `yaml:"url" json:"url,omitempty"`
	// User name for registry acces
	User string `yaml:"user" json:"user,omitempty"`
	// Password for registry access
	Password string `yaml:"password" json:"password,omitempty"`
}

type RKESystemImages struct {
	// etcd image
	Etcd string `yaml:"etcd" json:"etcd,omitempty" norman:"default=quay.io/coreos/etcd:v3.0.17"`
	// Alpine image
	Alpine string `yaml:"alpine" json:"alpine,omitempty" norman:"default=alpine"`
	// rke-nginx-proxy image
	NginxProxy string `yaml:"nginx_proxy" json:"nginxProxy,omitempty" norman:"default=rancher/rke-nginx-proxy:v0.1.1"`
	// rke-cert-deployer image
	CertDownloader string `yaml:"cert_downloader" json:"certDownloader,omitempty" norman:"default=rancher/rke-cert-deployer:v0.1.1"`
	// rke-service-sidekick image
	KubernetesServicesSidecar string `yaml:"kubernetes_services_sidecar" json:"kubernetesServicesSidecar,omitempty" norman:"default=rancher/rke-service-sidekick:v0.1.0"`
	// KubeDNS image
	KubeDNS string `yaml:"kubedns" json:"kubedns,omitempty" norman:"default=gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.5"`
	// DNSMasq image
	DNSmasq string `yaml:"dnsmasq" json:"dnsmasq,omitempty" norman:"default=gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.5"`
	// KubeDNS side car image
	KubeDNSSidecar string `yaml:"kubedns_sidecar" json:"kubednsSidecar,omitempty" norman:"default=gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.5"`
	// KubeDNS autoscaler image
	KubeDNSAutoscaler string `yaml:"kubedns_autoscaler" json:"kubednsAutoscaler,omitempty" norman:"default=gcr.io/google_containers/cluster-proportional-autoscaler-amd64:1.0.0"`
	// Kubernetes image
	Kubernetes string `yaml:"kubernetes" json:"kubernetes,omitempty" norman:"default=rancher/k8s:v1.8.5-rancher4"`
	// Flannel image
	Flannel string `yaml:"flannel" json:"flannel,omitempty"`
	// Flannel CNI image
	FlannelCNI string `yaml:"flannel_cni" json:"flannelCni,omitempty"`
	// Calico Node image
	CalicoNode string `yaml:"calico_node" json:"calicoNode,omitempty"`
	// Calico CNI image
	CalicoCNI string `yaml:"calico_cni" json:"calicoCni,omitempty"`
	// Calico Controllers image
	CalicoControllers string `yaml:"calico_controllers" json:"calicoControllers,omitempty"`
	// Calicoctl image
	CalicoCtl string `yaml:"calico_ctl" json:"calicoCtl,omitempty"`
	// Canal Node Image
	CanalNode string `yaml:"canal_node" json:"canalNode,omitempty"`
	// Canal CNI image
	CanalCNI string `yaml:"canal_cni" json:"canalCni,omitempty"`
	//CanalFlannel image
	CanalFlannel string `yaml:"canal_flannel" json:"canalFlannel,omitempty"`
	// Weave Node image
	WeaveNode string `yaml:"wave_node" json:"weaveNode,omitempty"`
	// Weave CNI image
	WeaveCNI string `yaml:"weave_cni" json:"weaveCni,omitempty"`
	// Pod infra container image
	PodInfraContainer string `yaml:"pod_infra_container" json:"podInfraContainer,omitempty"`
}

type RKEConfigNode struct {
	// Name of the host provisioned via docker machine
	MachineName string `yaml:"machine_name,omitempty" json:"machineName,omitempty" norman:"type=reference[machine]"`
	// IP or FQDN that is fully resolvable and used for SSH communication
	Address string `yaml:"address" json:"address,omitempty"`
	// Optional - Internal address that will be used for components communication
	InternalAddress string `yaml:"internal_address" json:"internalAddress,omitempty"`
	// Node role in kubernetes cluster (controlplane, worker, or etcd)
	Role []string `yaml:"role" json:"role,omitempty" norman:"type=array[enum],options=etcd|worker|controlplane"`
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
	// Node Labels
	Labels map[string]string `yaml:"labels" json:"labels,omitempty"`
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
	// Enabled/Disable PodSecurityPolicy
	PodSecurityPolicy bool `yaml:"pod_security_policy" json:"podSecurityPolicy,omitempty"`
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
	// Fail if swap is enabled
	FailSwapOn bool `yaml:"fail_swap_on" json:"failSwapOn,omitempty"`
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

type AuthnConfig struct {
	// Authentication strategy that will be used in kubernetes cluster
	Strategy string `yaml:"strategy" json:"strategy,omitempty"`
	// Authentication options
	Options map[string]string `yaml:"options" json:"options,omitempty"`
}

type AuthzConfig struct {
	// Authorization mode used by kubernetes
	Mode string `yaml:"mode" json:"mode,omitempty"`
	// Authorization mode options
	Options map[string]string `yaml:"options" json:"options,omitempty"`
}

type IngressConfig struct {
	// Ingress controller type used by kubernetes
	Provider string `yaml:"provider" json:"provider,omitempty"`
	// Ingress controller options
	Options map[string]string `yaml:"options" json:"options,omitempty"`
	// NodeSelector key pair
	NodeSelector map[string]string `yaml:"node_selector" json:"nodeSelector,omitempty"`
}
