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
	// List of urls or paths for addons
	AddonsInclude []string `yaml:"addons_include" json:"addonsInclude,omitempty"`
	// List of images used internally for proxy, cert downlaod and kubedns
	SystemImages RKESystemImages `yaml:"system_images" json:"systemImages,omitempty"`
	// SSH Private Key Path
	SSHKeyPath string `yaml:"ssh_key_path" json:"sshKeyPath,omitempty"`
	// SSH Agent Auth enable
	SSHAgentAuth bool `yaml:"ssh_agent_auth" json:"sshAgentAuth"`
	// Authorization mode configuration used in the cluster
	Authorization AuthzConfig `yaml:"authorization" json:"authorization,omitempty"`
	// Enable/disable strict docker version checking
	IgnoreDockerVersion bool `yaml:"ignore_docker_version" json:"ignoreDockerVersion" norman:"default=true"`
	// Kubernetes version to use (if kubernetes image is specifed, image version takes precedence)
	Version string `yaml:"kubernetes_version" json:"kubernetesVersion,omitempty"`
	// List of private registries and their credentials
	PrivateRegistries []PrivateRegistry `yaml:"private_registries" json:"privateRegistries,omitempty"`
	// Ingress controller used in the cluster
	Ingress IngressConfig `yaml:"ingress" json:"ingress,omitempty"`
	// Cluster Name used in the kube config
	ClusterName string `yaml:"cluster_name" json:"clusterName,omitempty"`
	// Cloud Provider options
	CloudProvider CloudProvider `yaml:"cloud_provider" json:"cloudProvider,omitempty"`
	// kubernetes directory path
	PrefixPath string `yaml:"prefix_path" json:"prefixPath,omitempty"`
	// Timeout in seconds for status check on addon deployment jobs
	AddonJobTimeout int `yaml:"addon_job_timeout" json:"addonJobTimeout,omitempty" norman:"default=30"`
	// Bastion/Jump Host configuration
	BastionHost BastionHost `yaml:"bastion_host" json:"bastionHost,omitempty"`
	// Monitoring Config
	Monitoring MonitoringConfig `yaml:"monitoring" json:"monitoring,omitempty"`
	// Rotating Certificates Option
	RotateCertificates *RotateCertificates `yaml:"rotate_certificates,omitempty" json:"rotateCertificates,omitempty"`
}

type BastionHost struct {
	// Address of Bastion Host
	Address string `yaml:"address" json:"address,omitempty"`
	// SSH Port of Bastion Host
	Port string `yaml:"port" json:"port,omitempty"`
	// ssh User to Bastion Host
	User string `yaml:"user" json:"user,omitempty"`
	// SSH Agent Auth enable
	SSHAgentAuth bool `yaml:"ssh_agent_auth,omitempty" json:"sshAgentAuth,omitempty"`
	// SSH Private Key
	SSHKey string `yaml:"ssh_key" json:"sshKey,omitempty" norman:"type=password"`
	// SSH Private Key Path
	SSHKeyPath string `yaml:"ssh_key_path" json:"sshKeyPath,omitempty"`
}

type PrivateRegistry struct {
	// URL for the registry
	URL string `yaml:"url" json:"url,omitempty"`
	// User name for registry acces
	User string `yaml:"user" json:"user,omitempty"`
	// Password for registry access
	Password string `yaml:"password" json:"password,omitempty" norman:"type=password"`
	// Default registry
	IsDefault bool `yaml:"is_default" json:"isDefault,omitempty"`
}

type RKESystemImages struct {
	// etcd image
	Etcd string `yaml:"etcd" json:"etcd,omitempty"`
	// Alpine image
	Alpine string `yaml:"alpine" json:"alpine,omitempty"`
	// rke-nginx-proxy image
	NginxProxy string `yaml:"nginx_proxy" json:"nginxProxy,omitempty"`
	// rke-cert-deployer image
	CertDownloader string `yaml:"cert_downloader" json:"certDownloader,omitempty"`
	// rke-service-sidekick image
	KubernetesServicesSidecar string `yaml:"kubernetes_services_sidecar" json:"kubernetesServicesSidecar,omitempty"`
	// KubeDNS image
	KubeDNS string `yaml:"kubedns" json:"kubedns,omitempty"`
	// DNSMasq image
	DNSmasq string `yaml:"dnsmasq" json:"dnsmasq,omitempty"`
	// KubeDNS side car image
	KubeDNSSidecar string `yaml:"kubedns_sidecar" json:"kubednsSidecar,omitempty"`
	// KubeDNS autoscaler image
	KubeDNSAutoscaler string `yaml:"kubedns_autoscaler" json:"kubednsAutoscaler,omitempty"`
	// Kubernetes image
	Kubernetes string `yaml:"kubernetes" json:"kubernetes,omitempty"`
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
	WeaveNode string `yaml:"weave_node" json:"weaveNode,omitempty"`
	// Weave CNI image
	WeaveCNI string `yaml:"weave_cni" json:"weaveCni,omitempty"`
	// Pod infra container image
	PodInfraContainer string `yaml:"pod_infra_container" json:"podInfraContainer,omitempty"`
	// Ingress Controller image
	Ingress string `yaml:"ingress" json:"ingress,omitempty"`
	// Ingress Controller Backend image
	IngressBackend string `yaml:"ingress_backend" json:"ingressBackend,omitempty"`
	// Metrics Server image
	MetricsServer string `yaml:"metrics_server" json:"metricsServer,omitempty"`
}

type RKEConfigNode struct {
	// Name of the host provisioned via docker machine
	NodeName string `yaml:"nodeName,omitempty" json:"nodeName,omitempty" norman:"type=reference[node]"`
	// IP or FQDN that is fully resolvable and used for SSH communication
	Address string `yaml:"address" json:"address,omitempty"`
	// Port used for SSH communication
	Port string `yaml:"port" json:"port,omitempty"`
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
	// SSH Agent Auth enable
	SSHAgentAuth bool `yaml:"ssh_agent_auth,omitempty" json:"sshAgentAuth,omitempty"`
	// SSH Private Key
	SSHKey string `yaml:"ssh_key" json:"sshKey,omitempty" norman:"type=password"`
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
	// List of etcd urls
	ExternalURLs []string `yaml:"external_urls" json:"externalUrls,omitempty"`
	// External CA certificate
	CACert string `yaml:"ca_cert" json:"caCert,omitempty"`
	// External Client certificate
	Cert string `yaml:"cert" json:"cert,omitempty"`
	// External Client key
	Key string `yaml:"key" json:"key,omitempty"`
	// External etcd prefix
	Path string `yaml:"path" json:"path,omitempty"`
	// Etcd Recurring snapshot Service
	Snapshot *bool `yaml:"snapshot" json:"snapshot,omitempty" norman:"default=true"`
	// Etcd snapshot Retention period
	Retention string `yaml:"retention" json:"retention,omitempty" norman:"default=72h"`
	// Etcd snapshot Creation period
	Creation string `yaml:"creation" json:"creation,omitempty" norman:"default=12h"`
}

type KubeAPIService struct {
	// Base service properties
	BaseService `yaml:",inline" json:",inline"`
	// Virtual IP range that will be used by Kubernetes services
	ServiceClusterIPRange string `yaml:"service_cluster_ip_range" json:"serviceClusterIpRange,omitempty"`
	// Port range for services defined with NodePort type
	ServiceNodePortRange string `yaml:"service_node_port_range" json:"serviceNodePortRange,omitempty" norman:"default=30000-32767"`
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
	// Extra binds added to the nodes
	ExtraBinds []string `yaml:"extra_binds" json:"extraBinds,omitempty"`
	// this is to provide extra env variable to the docker container running kubernetes service
	ExtraEnv []string `yaml:"extra_env" json:"extraEnv,omitempty"`
}

type NetworkConfig struct {
	// Network Plugin That will be used in kubernetes cluster
	Plugin string `yaml:"plugin" json:"plugin,omitempty" norman:"default=canal"`
	// Plugin options to configure network properties
	Options map[string]string `yaml:"options" json:"options,omitempty"`
	// CalicoNetworkProvider
	CalicoNetworkProvider *CalicoNetworkProvider `yaml:",omitempty" json:"calicoNetworkProvider,omitempty"`
	// CanalNetworkProvider
	CanalNetworkProvider *CanalNetworkProvider `yaml:",omitempty" json:"canalNetworkProvider,omitempty"`
	// FlannelNetworkProvider
	FlannelNetworkProvider *FlannelNetworkProvider `yaml:",omitempty" json:"flannelNetworkProvider,omitempty"`
	// WeaveNetworkProvider
	WeaveNetworkProvider *WeaveNetworkProvider `yaml:",omitempty" json:"weaveNetworkProvider,omitempty"`
}

type AuthnConfig struct {
	// Authentication strategy that will be used in kubernetes cluster
	Strategy string `yaml:"strategy" json:"strategy,omitempty" norman:"default=x509"`
	// Authentication options
	Options map[string]string `yaml:"options" json:"options,omitempty"`
	// List of additional hostnames and IPs to include in the api server PKI cert
	SANs []string `yaml:"sans" json:"sans,omitempty"`
}

type AuthzConfig struct {
	// Authorization mode used by kubernetes
	Mode string `yaml:"mode" json:"mode,omitempty"`
	// Authorization mode options
	Options map[string]string `yaml:"options" json:"options,omitempty"`
}

type IngressConfig struct {
	// Ingress controller type used by kubernetes
	Provider string `yaml:"provider" json:"provider,omitempty" norman:"default=nginx"`
	// Ingress controller options
	Options map[string]string `yaml:"options" json:"options,omitempty"`
	// NodeSelector key pair
	NodeSelector map[string]string `yaml:"node_selector" json:"nodeSelector,omitempty"`
	// Ingress controller extra arguments
	ExtraArgs map[string]string `yaml:"extra_args" json:"extraArgs,omitempty"`
}

type RKEPlan struct {
	// List of node Plans
	Nodes []RKEConfigNodePlan `json:"nodes,omitempty"`
}

type RKEConfigNodePlan struct {
	// Node address
	Address string `json:"address,omitempty"`
	// map of named processes that should run on the node
	Processes map[string]Process `json:"processes,omitempty"`
	// List of portchecks that should be open on the node
	PortChecks []PortCheck `json:"portChecks,omitempty"`
	// List of files to deploy on the node
	Files []File `json:"files,omitempty"`
	// Node Annotations
	Annotations map[string]string `json:"annotations,omitempty"`
	// Node Labels
	Labels map[string]string `json:"labels,omitempty"`
}

type Process struct {
	// Process name, this should be the container name
	Name string `json:"name,omitempty"`
	// Process Entrypoint command
	Command []string `json:"command,omitempty"`
	// Process args
	Args []string `json:"args,omitempty"`
	// Environment variables list
	Env []string `json:"env,omitempty"`
	// Process docker image
	Image string `json:"image,omitempty"`
	//AuthConfig for image private registry
	ImageRegistryAuthConfig string `json:"imageRegistryAuthConfig,omitempty"`
	// Process docker image VolumesFrom
	VolumesFrom []string `json:"volumesFrom,omitempty"`
	// Process docker container bind mounts
	Binds []string `json:"binds,omitempty"`
	// Process docker container netwotk mode
	NetworkMode string `json:"networkMode,omitempty"`
	// Process container restart policy
	RestartPolicy string `json:"restartPolicy,omitempty"`
	// Process container pid mode
	PidMode string `json:"pidMode,omitempty"`
	// Run process in privileged container
	Privileged bool `json:"privileged,omitempty"`
	// Process healthcheck
	HealthCheck HealthCheck `json:"healthCheck,omitempty"`
	// Process docker container Labels
	Labels map[string]string `json:"labels,omitempty"`
	// Process docker publish container's port to host
	Publish []string `json:"publish,omitempty"`
}

type HealthCheck struct {
	// Healthcheck URL
	URL string `json:"url,omitempty"`
}

type PortCheck struct {
	// Portcheck address to check.
	Address string `json:"address,omitempty"`
	// Port number
	Port int `json:"port,omitempty"`
	// Port Protocol
	Protocol string `json:"protocol,omitempty"`
}

type CloudProvider struct {
	// Name of the Cloud Provider
	Name string `yaml:"name" json:"name,omitempty"`
	// AWSCloudProvider
	AWSCloudProvider *AWSCloudProvider `yaml:"awsCloudProvider,omitempty" json:"awsCloudProvider,omitempty"`
	// AzureCloudProvider
	AzureCloudProvider *AzureCloudProvider `yaml:"azureCloudProvider,omitempty" json:"azureCloudProvider,omitempty"`
	// OpenstackCloudProvider
	OpenstackCloudProvider *OpenstackCloudProvider `yaml:"openstackCloudProvider,omitempty" json:"openstackCloudProvider,omitempty"`
	// VsphereCloudProvider
	VsphereCloudProvider *VsphereCloudProvider `yaml:"vsphereCloudProvider,omitempty" json:"vsphereCloudProvider,omitempty"`
	// CustomCloudProvider is a multiline string that represent a custom cloud config file
	CustomCloudProvider string `yaml:"customCloudProvider,omitempty" json:"customCloudProvider,omitempty"`
}

type CalicoNetworkProvider struct {
	// Cloud provider type used with calico
	CloudProvider string `json:"cloudProvider"`
}

type FlannelNetworkProvider struct {
	// Alternate cloud interface for flannel
	Iface string `json:"iface"`
}

type CanalNetworkProvider struct {
	FlannelNetworkProvider `yaml:",inline" json:",inline"`
}

type WeaveNetworkProvider struct {
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

type KubernetesServicesOptions struct {
	// Additional options passed to KubeAPI
	KubeAPI map[string]string `json:"kubeapi"`
	// Additional options passed to Kubelet
	Kubelet map[string]string `json:"kubelet"`
	// Additional options passed to Kubeproxy
	Kubeproxy map[string]string `json:"kubeproxy"`
	// Additional options passed to KubeController
	KubeController map[string]string `json:"kubeController"`
	// Additional options passed to Scheduler
	Scheduler map[string]string `json:"scheduler"`
}

// VsphereCloudProvider options
type VsphereCloudProvider struct {
	Global        GlobalVsphereOpts              `json:"global,omitempty" yaml:"global,omitempty" ini:"Global,omitempty"`
	VirtualCenter map[string]VirtualCenterConfig `json:"virtualCenter,omitempty" yaml:"virtual_center,omitempty" ini:"VirtualCenter,omitempty"`
	Network       NetworkVshpereOpts             `json:"network,omitempty" yaml:"network,omitempty" ini:"Network,omitempty"`
	Disk          DiskVsphereOpts                `json:"disk,omitempty" yaml:"disk,omitempty" ini:"Disk,omitempty"`
	Workspace     WorkspaceVsphereOpts           `json:"workspace,omitempty" yaml:"workspace,omitempty" ini:"Workspace,omitempty"`
}

type GlobalVsphereOpts struct {
	User              string `json:"user,omitempty" yaml:"user,omitempty" ini:"user,omitempty"`
	Password          string `json:"password,omitempty" yaml:"password,omitempty" ini:"password,omitempty"`
	VCenterIP         string `json:"server,omitempty" yaml:"server,omitempty" ini:"server,omitempty"`
	VCenterPort       string `json:"port,omitempty" yaml:"port,omitempty" ini:"port,omitempty"`
	InsecureFlag      bool   `json:"insecure-flag,omitempty" yaml:"insecure-flag,omitempty" ini:"insecure-flag,omitempty"`
	Datacenter        string `json:"datacenter,omitempty" yaml:"datacenter,omitempty" ini:"datacenter,omitempty"`
	Datacenters       string `json:"datacenters,omitempty" yaml:"datacenters,omitempty" ini:"datacenters,omitempty"`
	DefaultDatastore  string `json:"datastore,omitempty" yaml:"datastore,omitempty" ini:"datastore,omitempty"`
	WorkingDir        string `json:"working-dir,omitempty" yaml:"working-dir,omitempty" ini:"working-dir,omitempty"`
	RoundTripperCount int    `json:"soap-roundtrip-count,omitempty" yaml:"soap-roundtrip-count,omitempty" ini:"soap-roundtrip-count,omitempty"`
	VMUUID            string `json:"vm-uuid,omitempty" yaml:"vm-uuid,omitempty" ini:"vm-uuid,omitempty"`
	VMName            string `json:"vm-name,omitempty" yaml:"vm-name,omitempty" ini:"vm-name,omitempty"`
}

type VirtualCenterConfig struct {
	User              string `json:"user,omitempty" yaml:"user,omitempty" ini:"user,omitempty"`
	Password          string `json:"password,omitempty" yaml:"password,omitempty" ini:"password,omitempty"`
	VCenterPort       string `json:"port,omitempty" yaml:"port,omitempty" ini:"port,omitempty"`
	Datacenters       string `json:"datacenters,omitempty" yaml:"datacenters,omitempty" ini:"datacenters,omitempty"`
	RoundTripperCount int    `json:"soap-roundtrip-count,omitempty" yaml:"soap-roundtrip-count,omitempty" ini:"soap-roundtrip-count,omitempty"`
}

type NetworkVshpereOpts struct {
	PublicNetwork string `json:"public-network,omitempty" yaml:"public-network,omitempty" ini:"public-network,omitempty"`
}

type DiskVsphereOpts struct {
	SCSIControllerType string `json:"scsicontrollertype,omitempty" yaml:"scsicontrollertype,omitempty" ini:"scsicontrollertype,omitempty"`
}

type WorkspaceVsphereOpts struct {
	VCenterIP        string `json:"server,omitempty" yaml:"server,omitempty" ini:"server,omitempty"`
	Datacenter       string `json:"datacenter,omitempty" yaml:"datacenter,omitempty" ini:"datacenter,omitempty"`
	Folder           string `json:"folder,omitempty" yaml:"folder,omitempty" ini:"folder,omitempty"`
	DefaultDatastore string `json:"default-datastore,omitempty" yaml:"default-datastore,omitempty" ini:"default-datastore,omitempty"`
	ResourcePoolPath string `json:"resourcepool-path,omitempty" yaml:"resourcepool-path,omitempty" ini:"resourcepool-path,omitempty"`
}

// OpenstackCloudProvider options
type OpenstackCloudProvider struct {
	Global       GlobalOpenstackOpts       `json:"global" yaml:"global" ini:"Global,omitempty"`
	LoadBalancer LoadBalancerOpenstackOpts `json:"loadBalancer" yaml:"load_balancer" ini:"LoadBalancer,omitempty"`
	BlockStorage BlockStorageOpenstackOpts `json:"blockStorage" yaml:"block_storage" ini:"BlockStorage,omitempty"`
	Route        RouteOpenstackOpts        `json:"route" yaml:"route" ini:"Route,omitempty"`
	Metadata     MetadataOpenstackOpts     `json:"metadata" yaml:"metadata" ini:"Metadata,omitempty"`
}

type GlobalOpenstackOpts struct {
	AuthURL    string `json:"auth-url" yaml:"auth-url" ini:"auth-url,omitempty"`
	Username   string `json:"username" yaml:"username" ini:"username,omitempty"`
	UserID     string `json:"user-id" yaml:"user-id" ini:"user-id,omitempty"`
	Password   string `json:"password" yaml:"password" ini:"password,omitempty"`
	TenantID   string `json:"tenant-id" yaml:"tenant-id" ini:"tenant-id,omitempty"`
	TenantName string `json:"tenant-name" yaml:"tenant-name" ini:"tenant-name,omitempty"`
	TrustID    string `json:"trust-id" yaml:"trust-id" ini:"trust-id,omitempty"`
	DomainID   string `json:"domain-id" yaml:"domain-id" ini:"domain-id,omitempty"`
	DomainName string `json:"domain-name" yaml:"domain-name" ini:"domain-name,omitempty"`
	Region     string `json:"region" yaml:"region" ini:"region,omitempty"`
	CAFile     string `json:"ca-file" yaml:"ca-file" ini:"ca-file,omitempty"`
}

type LoadBalancerOpenstackOpts struct {
	LBVersion            string `json:"lb-version" yaml:"lb-version" ini:"lb-version,omitempty"`                            // overrides autodetection. Only support v2.
	UseOctavia           bool   `json:"use-octavia" yaml:"use-octavia" ini:"use-octavia,omitempty"`                         // uses Octavia V2 service catalog endpoint
	SubnetID             string `json:"subnet-id" yaml:"subnet-id" ini:"subnet-id,omitempty"`                               // overrides autodetection.
	FloatingNetworkID    string `json:"floating-network-id" yaml:"floating-network-id" ini:"floating-network-id,omitempty"` // If specified, will create floating ip for loadbalancer, or do not create floating ip.
	LBMethod             string `json:"lb-method" yaml:"lb-method" ini:"lb-method,omitempty"`                               // default to ROUND_ROBIN.
	LBProvider           string `json:"lb-provider" yaml:"lb-provider" ini:"lb-provider,omitempty"`
	CreateMonitor        bool   `json:"create-monitor" yaml:"create-monitor" ini:"create-monitor,omitempty"`
	MonitorDelay         int    `json:"monitor-delay" yaml:"monitor-delay" ini:"monitor-delay,omitempty"`
	MonitorTimeout       int    `json:"monitor-timeout" yaml:"monitor-timeout" ini:"monitor-timeout,omitempty"`
	MonitorMaxRetries    int    `json:"monitor-max-retries" yaml:"monitor-max-retries" ini:"monitor-max-retries,omitempty"`
	ManageSecurityGroups bool   `json:"manage-security-groups" yaml:"manage-security-groups" ini:"manage-security-groups,omitempty"`
}

type BlockStorageOpenstackOpts struct {
	BSVersion       string `json:"bs-version" yaml:"bs-version" ini:"bs-version,omitempty"`                      // overrides autodetection. v1 or v2. Defaults to auto
	TrustDevicePath bool   `json:"trust-device-path" yaml:"trust-device-path" ini:"trust-device-path,omitempty"` // See Issue #33128
	IgnoreVolumeAZ  bool   `json:"ignore-volume-az" yaml:"ignore-volume-az" ini:"ignore-volume-az,omitempty"`
}

type RouteOpenstackOpts struct {
	RouterID string `json:"router-id" yaml:"router-id" ini:"router-id,omitempty"` // required
}

type MetadataOpenstackOpts struct {
	SearchOrder    string `json:"search-order" yaml:"search-order" ini:"search-order,omitempty"`
	RequestTimeout int    `json:"request-timeout" yaml:"request-timeout" ini:"request-timeout,omitempty"`
}

// AzureCloudProvider options
type AzureCloudProvider struct {
	// The cloud environment identifier. Takes values from https://github.com/Azure/go-autorest/blob/ec5f4903f77ed9927ac95b19ab8e44ada64c1356/autorest/azure/environments.go#L13
	Cloud string `json:"cloud" yaml:"cloud"`
	// The AAD Tenant ID for the Subscription that the cluster is deployed in
	TenantID string `json:"tenantId" yaml:"tenantId"`
	// The ID of the Azure Subscription that the cluster is deployed in
	SubscriptionID string `json:"subscriptionId" yaml:"subscriptionId"`
	// The name of the resource group that the cluster is deployed in
	ResourceGroup string `json:"resourceGroup" yaml:"resourceGroup"`
	// The location of the resource group that the cluster is deployed in
	Location string `json:"location" yaml:"location"`
	// The name of the VNet that the cluster is deployed in
	VnetName string `json:"vnetName" yaml:"vnetName"`
	// The name of the resource group that the Vnet is deployed in
	VnetResourceGroup string `json:"vnetResourceGroup" yaml:"vnetResourceGroup"`
	// The name of the subnet that the cluster is deployed in
	SubnetName string `json:"subnetName" yaml:"subnetName"`
	// The name of the security group attached to the cluster's subnet
	SecurityGroupName string `json:"securityGroupName" yaml:"securityGroupName"`
	// (Optional in 1.6) The name of the route table attached to the subnet that the cluster is deployed in
	RouteTableName string `json:"routeTableName" yaml:"routeTableName"`
	// (Optional) The name of the availability set that should be used as the load balancer backend
	// If this is set, the Azure cloudprovider will only add nodes from that availability set to the load
	// balancer backend pool. If this is not set, and multiple agent pools (availability sets) are used, then
	// the cloudprovider will try to add all nodes to a single backend pool which is forbidden.
	// In other words, if you use multiple agent pools (availability sets), you MUST set this field.
	PrimaryAvailabilitySetName string `json:"primaryAvailabilitySetName" yaml:"primaryAvailabilitySetName"`
	// The type of azure nodes. Candidate valudes are: vmss and standard.
	// If not set, it will be default to standard.
	VMType string `json:"vmType" yaml:"vmType"`
	// The name of the scale set that should be used as the load balancer backend.
	// If this is set, the Azure cloudprovider will only add nodes from that scale set to the load
	// balancer backend pool. If this is not set, and multiple agent pools (scale sets) are used, then
	// the cloudprovider will try to add all nodes to a single backend pool which is forbidden.
	// In other words, if you use multiple agent pools (scale sets), you MUST set this field.
	PrimaryScaleSetName string `json:"primaryScaleSetName" yaml:"primaryScaleSetName"`
	// The ClientID for an AAD application with RBAC access to talk to Azure RM APIs
	AADClientID string `json:"aadClientId" yaml:"aadClientId"`
	// The ClientSecret for an AAD application with RBAC access to talk to Azure RM APIs
	AADClientSecret string `json:"aadClientSecret" yaml:"aadClientSecret"`
	// The path of a client certificate for an AAD application with RBAC access to talk to Azure RM APIs
	AADClientCertPath string `json:"aadClientCertPath" yaml:"aadClientCertPath"`
	// The password of the client certificate for an AAD application with RBAC access to talk to Azure RM APIs
	AADClientCertPassword string `json:"aadClientCertPassword" yaml:"aadClientCertPassword"`
	// Enable exponential backoff to manage resource request retries
	CloudProviderBackoff bool `json:"cloudProviderBackoff" yaml:"cloudProviderBackoff"`
	// Backoff retry limit
	CloudProviderBackoffRetries int `json:"cloudProviderBackoffRetries" yaml:"cloudProviderBackoffRetries"`
	// Backoff exponent
	CloudProviderBackoffExponent int `json:"cloudProviderBackoffExponent" yaml:"cloudProviderBackoffExponent"`
	// Backoff duration
	CloudProviderBackoffDuration int `json:"cloudProviderBackoffDuration" yaml:"cloudProviderBackoffDuration"`
	// Backoff jitter
	CloudProviderBackoffJitter int `json:"cloudProviderBackoffJitter" yaml:"cloudProviderBackoffJitter"`
	// Enable rate limiting
	CloudProviderRateLimit bool `json:"cloudProviderRateLimit" yaml:"cloudProviderRateLimit"`
	// Rate limit QPS
	CloudProviderRateLimitQPS int `json:"cloudProviderRateLimitQPS" yaml:"cloudProviderRateLimitQPS"`
	// Rate limit Bucket Size
	CloudProviderRateLimitBucket int `json:"cloudProviderRateLimitBucket" yaml:"cloudProviderRateLimitBucket"`
	// Use instance metadata service where possible
	UseInstanceMetadata bool `json:"useInstanceMetadata" yaml:"useInstanceMetadata"`
	// Use managed service identity for the virtual machine to access Azure ARM APIs
	UseManagedIdentityExtension bool `json:"useManagedIdentityExtension" yaml:"useManagedIdentityExtension"`
	// Maximum allowed LoadBalancer Rule Count is the limit enforced by Azure Load balancer
	MaximumLoadBalancerRuleCount int `json:"maximumLoadBalancerRuleCount" yaml:"maximumLoadBalancerRuleCount"`
}

// AWSCloudProvider options
type AWSCloudProvider struct {
}

type MonitoringConfig struct {
	// Monitoring server provider
	Provider string `yaml:"provider" json:"provider,omitempty" norman:"default=metrics-server"`
	// Metrics server options
	Options map[string]string `yaml:"options" json:"options,omitempty"`
}

type RotateCertificates struct {
	// Rotate CA Certificates
	CACertificates bool `json:"caCertificates,omitempty"`
	// Services to rotate their certs
	Services []string `json:"services,omitempty" norman:"type=enum,options=etcd|kubelet|kube-apiserver|kube-proxy|kube-scheduler|kube-controller-manager"`
}
