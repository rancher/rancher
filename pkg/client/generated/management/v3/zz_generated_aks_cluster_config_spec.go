package client

const (
	AKSClusterConfigSpecType                             = "aksClusterConfigSpec"
	AKSClusterConfigSpecFieldAuthBaseURL                 = "authBaseUrl"
	AKSClusterConfigSpecFieldAuthorizedIPRanges          = "authorizedIpRanges"
	AKSClusterConfigSpecFieldAzureCredentialSecret       = "azureCredentialSecret"
	AKSClusterConfigSpecFieldBaseURL                     = "baseUrl"
	AKSClusterConfigSpecFieldClusterName                 = "clusterName"
	AKSClusterConfigSpecFieldDNSPrefix                   = "dnsPrefix"
	AKSClusterConfigSpecFieldImported                    = "imported"
	AKSClusterConfigSpecFieldKubernetesVersion           = "kubernetesVersion"
	AKSClusterConfigSpecFieldLinuxAdminUsername          = "linuxAdminUsername"
	AKSClusterConfigSpecFieldLinuxSSHPublicKey           = "sshPublicKey"
	AKSClusterConfigSpecFieldLoadBalancerSKU             = "loadBalancerSku"
	AKSClusterConfigSpecFieldNetworkDNSServiceIP         = "dnsServiceIp"
	AKSClusterConfigSpecFieldNetworkDockerBridgeCIDR     = "dockerBridgeCidr"
	AKSClusterConfigSpecFieldNetworkPlugin               = "networkPlugin"
	AKSClusterConfigSpecFieldNetworkPodCIDR              = "podCidr"
	AKSClusterConfigSpecFieldNetworkPolicy               = "networkPolicy"
	AKSClusterConfigSpecFieldNetworkServiceCIDR          = "serviceCidr"
	AKSClusterConfigSpecFieldNodePools                   = "nodePools"
	AKSClusterConfigSpecFieldPrivateCluster              = "privateCluster"
	AKSClusterConfigSpecFieldResourceGroup               = "resourceGroup"
	AKSClusterConfigSpecFieldResourceLocation            = "resourceLocation"
	AKSClusterConfigSpecFieldSubnet                      = "subnet"
	AKSClusterConfigSpecFieldTags                        = "tags"
	AKSClusterConfigSpecFieldTenantID                    = "tenantId"
	AKSClusterConfigSpecFieldVirtualNetwork              = "virtualNetwork"
	AKSClusterConfigSpecFieldVirtualNetworkResourceGroup = "virtualNetworkResourceGroup"
	AKSClusterConfigSpecFieldWindowsAdminPassword        = "windowsAdminPassword"
	AKSClusterConfigSpecFieldWindowsAdminUsername        = "windowsAdminUsername"
)

type AKSClusterConfigSpec struct {
	AuthBaseURL                 string            `json:"authBaseUrl,omitempty" yaml:"authBaseUrl,omitempty"`
	AuthorizedIPRanges          []string          `json:"authorizedIpRanges,omitempty" yaml:"authorizedIpRanges,omitempty"`
	AzureCredentialSecret       string            `json:"azureCredentialSecret,omitempty" yaml:"azureCredentialSecret,omitempty"`
	BaseURL                     string            `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	ClusterName                 string            `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	DNSPrefix                   string            `json:"dnsPrefix,omitempty" yaml:"dnsPrefix,omitempty"`
	Imported                    bool              `json:"imported,omitempty" yaml:"imported,omitempty"`
	KubernetesVersion           string            `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	LinuxAdminUsername          string            `json:"linuxAdminUsername,omitempty" yaml:"linuxAdminUsername,omitempty"`
	LinuxSSHPublicKey           string            `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`
	LoadBalancerSKU             string            `json:"loadBalancerSku,omitempty" yaml:"loadBalancerSku,omitempty"`
	NetworkDNSServiceIP         string            `json:"dnsServiceIp,omitempty" yaml:"dnsServiceIp,omitempty"`
	NetworkDockerBridgeCIDR     string            `json:"dockerBridgeCidr,omitempty" yaml:"dockerBridgeCidr,omitempty"`
	NetworkPlugin               string            `json:"networkPlugin,omitempty" yaml:"networkPlugin,omitempty"`
	NetworkPodCIDR              string            `json:"podCidr,omitempty" yaml:"podCidr,omitempty"`
	NetworkPolicy               string            `json:"networkPolicy,omitempty" yaml:"networkPolicy,omitempty"`
	NetworkServiceCIDR          string            `json:"serviceCidr,omitempty" yaml:"serviceCidr,omitempty"`
	NodePools                   []AKSNodePool     `json:"nodePools,omitempty" yaml:"nodePools,omitempty"`
	PrivateCluster              *bool             `json:"privateCluster,omitempty" yaml:"privateCluster,omitempty"`
	ResourceGroup               string            `json:"resourceGroup,omitempty" yaml:"resourceGroup,omitempty"`
	ResourceLocation            string            `json:"resourceLocation,omitempty" yaml:"resourceLocation,omitempty"`
	Subnet                      string            `json:"subnet,omitempty" yaml:"subnet,omitempty"`
	Tags                        map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	TenantID                    string            `json:"tenantId,omitempty" yaml:"tenantId,omitempty"`
	VirtualNetwork              string            `json:"virtualNetwork,omitempty" yaml:"virtualNetwork,omitempty"`
	VirtualNetworkResourceGroup string            `json:"virtualNetworkResourceGroup,omitempty" yaml:"virtualNetworkResourceGroup,omitempty"`
	WindowsAdminPassword        string            `json:"windowsAdminPassword,omitempty" yaml:"windowsAdminPassword,omitempty"`
	WindowsAdminUsername        string            `json:"windowsAdminUsername,omitempty" yaml:"windowsAdminUsername,omitempty"`
}
