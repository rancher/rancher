package client

const (
	AzureKubernetesServiceConfigType                             = "azureKubernetesServiceConfig"
	AzureKubernetesServiceConfigFieldAdminUsername               = "adminUsername"
	AzureKubernetesServiceConfigFieldAgentDNSPrefix              = "agentDnsPrefix"
	AzureKubernetesServiceConfigFieldAgentPoolName               = "agentPoolName"
	AzureKubernetesServiceConfigFieldAgentVMSize                 = "agentVmSize"
	AzureKubernetesServiceConfigFieldBaseURL                     = "baseUrl"
	AzureKubernetesServiceConfigFieldClientID                    = "clientId"
	AzureKubernetesServiceConfigFieldClientSecret                = "clientSecret"
	AzureKubernetesServiceConfigFieldCount                       = "count"
	AzureKubernetesServiceConfigFieldDNSServiceIP                = "dnsServiceIp"
	AzureKubernetesServiceConfigFieldDockerBridgeCIDR            = "dockerBridgeCidr"
	AzureKubernetesServiceConfigFieldKubernetesVersion           = "kubernetesVersion"
	AzureKubernetesServiceConfigFieldLocation                    = "location"
	AzureKubernetesServiceConfigFieldMasterDNSPrefix             = "masterDnsPrefix"
	AzureKubernetesServiceConfigFieldOsDiskSizeGB                = "osDiskSizeGb"
	AzureKubernetesServiceConfigFieldResourceGroup               = "resourceGroup"
	AzureKubernetesServiceConfigFieldSSHPublicKeyContents        = "sshPublicKeyContents"
	AzureKubernetesServiceConfigFieldServiceCIDR                 = "serviceCidr"
	AzureKubernetesServiceConfigFieldSubnet                      = "subnet"
	AzureKubernetesServiceConfigFieldSubscriptionID              = "subscriptionId"
	AzureKubernetesServiceConfigFieldTag                         = "tags"
	AzureKubernetesServiceConfigFieldTenantID                    = "tenantId"
	AzureKubernetesServiceConfigFieldVirtualNetwork              = "virtualNetwork"
	AzureKubernetesServiceConfigFieldVirtualNetworkResourceGroup = "virtualNetworkResourceGroup"
)

type AzureKubernetesServiceConfig struct {
	AdminUsername               string            `json:"adminUsername,omitempty" yaml:"adminUsername,omitempty"`
	AgentDNSPrefix              string            `json:"agentDnsPrefix,omitempty" yaml:"agentDnsPrefix,omitempty"`
	AgentPoolName               string            `json:"agentPoolName,omitempty" yaml:"agentPoolName,omitempty"`
	AgentVMSize                 string            `json:"agentVmSize,omitempty" yaml:"agentVmSize,omitempty"`
	BaseURL                     string            `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	ClientID                    string            `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret                string            `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Count                       int64             `json:"count,omitempty" yaml:"count,omitempty"`
	DNSServiceIP                string            `json:"dnsServiceIp,omitempty" yaml:"dnsServiceIp,omitempty"`
	DockerBridgeCIDR            string            `json:"dockerBridgeCidr,omitempty" yaml:"dockerBridgeCidr,omitempty"`
	KubernetesVersion           string            `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	Location                    string            `json:"location,omitempty" yaml:"location,omitempty"`
	MasterDNSPrefix             string            `json:"masterDnsPrefix,omitempty" yaml:"masterDnsPrefix,omitempty"`
	OsDiskSizeGB                int64             `json:"osDiskSizeGb,omitempty" yaml:"osDiskSizeGb,omitempty"`
	ResourceGroup               string            `json:"resourceGroup,omitempty" yaml:"resourceGroup,omitempty"`
	SSHPublicKeyContents        string            `json:"sshPublicKeyContents,omitempty" yaml:"sshPublicKeyContents,omitempty"`
	ServiceCIDR                 string            `json:"serviceCidr,omitempty" yaml:"serviceCidr,omitempty"`
	Subnet                      string            `json:"subnet,omitempty" yaml:"subnet,omitempty"`
	SubscriptionID              string            `json:"subscriptionId,omitempty" yaml:"subscriptionId,omitempty"`
	Tag                         map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	TenantID                    string            `json:"tenantId,omitempty" yaml:"tenantId,omitempty"`
	VirtualNetwork              string            `json:"virtualNetwork,omitempty" yaml:"virtualNetwork,omitempty"`
	VirtualNetworkResourceGroup string            `json:"virtualNetworkResourceGroup,omitempty" yaml:"virtualNetworkResourceGroup,omitempty"`
}
