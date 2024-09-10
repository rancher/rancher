package aks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2019-10-01/containerservice"
	"github.com/Azure/azure-sdk-for-go/services/operationalinsights/mgmt/2020-08-01/operationalinsights"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2017-05-10/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/rancher/aks-operator/pkg/aks"
	"github.com/rancher/aks-operator/pkg/aks/services"
	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/options"
	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/util"
	"github.com/rancher/rancher/pkg/kontainer-engine/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var redactionRegex = regexp.MustCompile("\"(clientId|secret)\": \"(.*)\"")

type Driver struct {
	driverCapabilities types.Capabilities
}

type state struct {
	/**
	Azure Kubernetes Service API URI Parameters
	*/
	// SubscriptionID is a credential which uniquely identify Azure subscription. [requirement]
	SubscriptionID string `json:"subscriptionId"`
	// ResourceGroup specifies the cluster located int which resource group. [requirement]
	ResourceGroup string `json:"resourceGroup"`
	// Name specifies the cluster name. [requirement]
	Name string `json:"name"`

	/**
	Azure Kubernetes Service API Request Body
	*/
	// AzureADClientAppID specifies the client ID of Azure Active Directory. [optional when creating]
	AzureADClientAppID string `json:"addClientAppId,omitempty"`
	// AzureADServerAppID specifies the server ID of Azure Active Directory. [optional when creating]
	AzureADServerAppID string `json:"addServerAppId,omitempty"`
	// AzureADServerAppSecret specifies the server secret of Azure Active Directory. [optional when creating]
	AzureADServerAppSecret string `json:"addServerAppSecret,omitempty"`
	// AzureADTenantID specifies the tenant ID of Azure Active Directory. [optional when creating]
	AzureADTenantID string `json:"addTenantId,omitempty"`

	// AddonEnableHTTPApplicationRouting specifies to enable "httpApplicationRouting" addon or not. [optional]
	AddonEnableHTTPApplicationRouting bool `json:"enableHttpApplicationRouting,omitempty"`
	// AddonEnableMonitoring specifies to enable "monitoring" addon or not. [optional]
	AddonEnableMonitoring bool `json:"enableMonitoring,omitempty"`
	// LogAnalyticsWorkspaceResourceGroup specifies the Azure Log Analytics Workspace located int which resource group. [optional]
	LogAnalyticsWorkspaceResourceGroup string `json:"logAnalyticsWorkspaceResourceGroup,omitempty"`
	// LogAnalyticsWorkspace specifies an existing Azure Log Analytics Workspace  for "monitoring" addon. [optional]
	LogAnalyticsWorkspace string `json:"logAnalyticsWorkspace,omitempty"`

	// AgentDNSPrefix specifies the DNS prefix of the agent pool. [optional only when creating]
	AgentDNSPrefix string `json:"agentDnsPrefix,omitempty"`
	// AgentCount specifies the number of machines in the agent pool. [optional only when creating]
	AgentCount int64 `json:"count,omitempty"`
	// AgentMaxPods specifies the maximum number of pods that can run on a node. [optional only when creating]
	AgentMaxPods int64 `json:"maxPods,omitempty"`
	// AgentName specifies an unique name of the agent pool in the context of the subscription and resource group. [optional only when creating]
	AgentName string `json:"agentPoolName,omitempty"`
	// AgentOsdiskSizeGB specifies the disk size for every machine in the agent pool. [optional only when creating]
	AgentOsdiskSizeGB int64 `json:"agentOsdiskSize,omitempty"`
	// AgentVMSize specifies the VM size in the agent pool. [optional only when creating]
	AgentVMSize string `json:"agentVmSize,omitempty"`
	// LoadBalancerSku specifies the LoadBalancer SKU of the cluster. [optional only when creating]
	LoadBalancerSku string `json:"loadBalancerSku,omitempty"`
	// VirtualNetworkResourceGroup specifies the Azure Virtual Network located int which resource group. Composite of agent virtual network subnet ID. [optional only when creating]
	VirtualNetworkResourceGroup string `json:"virtualNetworkResourceGroup,omitempty"`
	// VirtualNetwork specifies an existing Azure Virtual Network. Composite of agent virtual network subnet ID. [optional only when creating]
	VirtualNetwork string `json:"virtualNetwork,omitempty"`
	// Subnet specifies an existing Azure Virtual Subnet. Composite of agent virtual network subnet ID. [optional only when creating]
	Subnet string `json:"subnet,omitempty"`

	// LinuxAdminUsername specifies the username to use for Linux VMs. [optional only when creating]
	LinuxAdminUsername string `json:"adminUsername,omitempty"`
	// LinuxSSHPublicKeyContents specifies the content of the SSH configuration for Linux VMs. [requirement only when creating]
	LinuxSSHPublicKeyContents string `json:"sshPublicKeyContents,omitempty"`

	// NetworkDNSServiceIP specifies an IP address assigned to the Kubernetes DNS service, it must be within the Kubernetes Service address range specified in `NetworkServiceCIDR`. [optional only when creating]
	NetworkDNSServiceIP string `json:"dnsServiceIp,omitempty"`
	// NetworkDockerBridgeCIDR specifies a CIDR notation IP range assigned to the Docker bridge network, it must not overlap with any Azure Subnet IP ranges or the Kubernetes Service address range. [optional only when creating]
	NetworkDockerBridgeCIDR string `json:"dockerBridgeCidr,omitempty"`
	// NetworkPlugin specifies the plugin used for Kubernetes network. [optional only when creating]
	NetworkPlugin string `json:"networkPlugin,omitempty"`
	// NetworkPolicy specifies the policy  used for Kubernetes network. [optional only when creating]
	NetworkPolicy string `json:"networkPolicy,omitempty"`
	// NetworkPodCIDR specifies a CIDR notation IP range from which to assign pod IPs when `NetworkPlugin` is using "kubenet". [optional only when creating]
	NetworkPodCIDR string `json:"podCidr,omitempty"`
	// NetworkServiceCIDR specifies a CIDR notation IP range from which to assign service cluster IPs, it must not overlap with any Azure Subnet IP ranges. [optional only when creating]
	NetworkServiceCIDR string `json:"serviceCidr,omitempty"`

	// Location specifies the cluster location. [requirement]
	Location string `json:"location,omitempty"`
	// DNSPrefix specifies the DNS prefix of the cluster. [optional only when creating]
	DNSPrefix string `json:"masterDnsPrefix,omitempty"`
	// KubernetesVersion specifies the Kubernetes version of the cluster. [optional]
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
	// Tags tag the cluster. [optional]
	Tags map[string]string `json:"tags,omitempty"`

	/**
	Azure Kubernetes Service API Metadata & Authentication
	*/
	// BaseURL specifies the Azure Resource management endpoint, it defaults "https://management.azure.com/". [requirement]
	BaseURL string `json:"baseUrl"`
	// AuthBaseURL specifies the Azure OAuth 2.0 authentication endpoint, it defaults "https://login.microsoftonline.com/". [requirement]
	AuthBaseURL string `json:"authBaseUrl"`
	// ClientID is a user ID for the Service Principal. [requirement]
	ClientID string `json:"clientId"`
	// ClientSecret is a plain-text password associated with the Service Principal. [requirement]
	ClientSecret string `json:"clientSecret"`
	// TenantID is a tenant ID for Azure OAuth 2.0 authentication. [optional only when creating]
	TenantID string `json:"tenantId,omitempty"`

	/**
	Rancher Parameters
	*/
	// DisplayName specifies cluster name displayed in Rancher UI. [optional only when creating]
	DisplayName string `json:"displayName,omitempty"`

	ClusterInfo types.ClusterInfo `json:"-"`
}

func NewDriver() types.Driver {
	driver := &Driver{
		driverCapabilities: types.Capabilities{
			Capabilities: make(map[int64]bool),
		},
	}

	driver.driverCapabilities.AddCapability(types.GetVersionCapability)
	driver.driverCapabilities.AddCapability(types.SetVersionCapability)
	driver.driverCapabilities.AddCapability(types.GetClusterSizeCapability)
	driver.driverCapabilities.AddCapability(types.SetClusterSizeCapability)

	return driver
}

// GetDriverCreateOptions implements driver interface
func (d *Driver) GetDriverCreateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}

	driverFlag.Options["subscription-id"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Subscription credentials which uniquely identify Microsoft Azure subscription.",
	}
	driverFlag.Options["resource-group"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The name of the 'Cluster' resource group.",
	}
	driverFlag.Options["name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The name of the 'Cluster' resource, and the internal name of the cluster in Rancher.",
	}

	driverFlag.Options["aad-client-app-id"] = &types.Flag{
		Type:  types.StringType,
		Usage: `The ID of an Azure Active Directory client application of type "Native". This application is for user login via kubectl.`,
	}
	driverFlag.Options["aad-server-app-id"] = &types.Flag{
		Type:  types.StringType,
		Usage: `The ID of an Azure Active Directory server application of type "Web app/API". This application represents the managed cluster's apiserver (Server application).`,
	}
	driverFlag.Options["aad-server-app-secret"] = &types.Flag{
		Type:  types.StringType,
		Usage: `The secret of an Azure Active Directory server application.`,
	}
	driverFlag.Options["aad-tenant-id"] = &types.Flag{
		Type:  types.StringType,
		Usage: `The ID of an Azure Active Directory tenant.`,
	}

	driverFlag.Options["enable-http-application-routing"] = &types.Flag{
		Type:  types.BoolType,
		Usage: `Enable the Kubernetes ingress with automatic public DNS name creation.`,
		Default: &types.Default{
			DefaultBool: false,
		},
	}
	driverFlag.Options["enable-monitoring"] = &types.Flag{
		Type:  types.BoolType,
		Usage: `Turn on Azure Log Analytics monitoring. Uses the Log Analytics "Default" workspace if it exists, else creates one. if using an existing workspace, specifies "log analytics workspace resource id".`,
		Default: &types.Default{
			DefaultBool: true,
		},
	}
	driverFlag.Options["log-analytics-workspace-resource-group"] = &types.Flag{
		Type:  types.StringType,
		Usage: `The resource group of an existing Azure Log Analytics Workspace to use for storing monitoring data. If not specified, uses the 'Cluster' resource group.`,
	}
	driverFlag.Options["log-analytics-workspace"] = &types.Flag{
		Type:  types.StringType,
		Usage: `The name of an existing Azure Log Analytics Workspace to use for storing monitoring data. If not specified, uses '{resource group}-{subscription id}-{location code}'.`,
	}

	driverFlag.Options["count"] = &types.Flag{
		Type:  types.IntType,
		Usage: "Number of machines (VMs) in the agent pool. Allowed values must be in the range of 1 to 100 (inclusive).",
		Default: &types.Default{
			DefaultInt: 1,
		},
	}
	driverFlag.Options["max-pods"] = &types.Flag{
		Type:  types.IntType,
		Usage: "Maximum number of pods that can run on a node.",
		Default: &types.Default{
			DefaultInt: 110,
		},
	}
	driverFlag.Options["agent-pool-name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Name for the agent pool, upto 12 alphanumeric characters.",
		Value: "agentpool0",
	}
	driverFlag.Options["agent-osdisk-size"] = &types.Flag{
		Type:  types.IntType,
		Usage: `GB size to be used to specify the disk for every machine in the agent pool. If you specify 0, it will apply the default according to the "agent vm size" specified.`,
	}

	driverFlag.Options["agent-vm-size"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Size of machine in the agent pool.",
		Value: string(containerservice.VMSizeTypesStandardD1V2),
	}
	driverFlag.Options["load-balancer-sku"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The LoadBalancer SKU of the cluster.",
	}
	driverFlag.Options["virtual-network-resource-group"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The resource group of an existing Azure Virtual Network. Composite of agent virtual network subnet ID.",
	}
	driverFlag.Options["virtual-network"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The name of an existing Azure Virtual Network. Composite of agent virtual network subnet ID.",
	}
	driverFlag.Options["subnet"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The name of an existing Azure Virtual Subnet. Composite of agent virtual network subnet ID.",
	}

	driverFlag.Options["admin-username"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The administrator username to use for Linux hosts.",
		Value: "azureuser",
	}
	driverFlag.Options["ssh-public-key-contents"] = &types.Flag{
		Type:  types.StringType,
		Usage: `Contents of the SSH public key used to authenticate with Linux hosts. Opposite to "ssh public key".`,
	}
	driverFlag.Options["dns-service-ip"] = &types.Flag{
		Type:  types.StringType,
		Usage: `An IP address assigned to the Kubernetes DNS service. It must be within the Kubernetes Service address range specified in "service cidr".`,
		Value: "10.0.0.10",
	}
	driverFlag.Options["docker-bridge-cidr"] = &types.Flag{
		Type:  types.StringType,
		Usage: `A CIDR notation IP range assigned to the Docker bridge network. It must not overlap with any Subnet IP ranges or the Kubernetes Service address range specified in "service cidr".`,
		Value: "172.17.0.1/16",
	}
	driverFlag.Options["network-plugin"] = &types.Flag{
		Type:  types.StringType,
		Usage: fmt.Sprintf(`Network plugin used for building Kubernetes network. Chooses from %v.`, containerservice.PossibleNetworkPluginValues()),
		Value: string(containerservice.Azure),
	}
	driverFlag.Options["network-policy"] = &types.Flag{
		Type:  types.StringType,
		Usage: fmt.Sprintf(`Network policy used for building Kubernetes network. Chooses from %v.`, containerservice.PossibleNetworkPolicyValues()),
	}
	driverFlag.Options["pod-cidr"] = &types.Flag{
		Type:  types.StringType,
		Usage: fmt.Sprintf(`A CIDR notation IP range from which to assign Kubernetes Pod IPs when "network plugin" is specified in %q.`, containerservice.Kubenet),
		Value: "172.244.0.0/16",
	}
	driverFlag.Options["service-cidr"] = &types.Flag{
		Type:  types.StringType,
		Usage: "A CIDR notation IP range from which to assign Kubernetes Service cluster IPs. It must not overlap with any Subnet IP ranges.",
		Value: "10.0.0.0/16",
	}

	driverFlag.Options["location"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Azure Kubernetes cluster location.",
		Value: "eastus",
	}
	driverFlag.Options["master-dns-prefix"] = &types.Flag{
		Type:  types.StringType,
		Usage: "DNS prefix to use the Kubernetes cluster control pane.",
	}
	driverFlag.Options["kubernetes-version"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Specify the version of Kubernetes.",
		Value: "1.11.5",
	}
	driverFlag.Options["tags"] = &types.Flag{
		Type:  types.StringSliceType,
		Usage: "Tags for Kubernetes cluster. For example, foo=bar.",
	}

	driverFlag.Options["base-url"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Different resource management API url to use.",
		Value: azure.PublicCloud.ResourceManagerEndpoint,
	}
	driverFlag.Options["auth-base-url"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Different authentication API url to use.",
		Value: azure.PublicCloud.ActiveDirectoryEndpoint,
	}
	driverFlag.Options["client-id"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Azure client ID to use.",
	}
	driverFlag.Options["client-secret"] = &types.Flag{
		Type:     types.StringType,
		Password: true,
		Usage:    `Azure client secret associated with the "client id".`,
	}
	driverFlag.Options["tenant-id"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Azure tenant ID to use.",
	}

	driverFlag.Options["display-name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The displayed name of the cluster in the Rancher UI.",
	}

	return &driverFlag, nil
}

// GetDriverUpdateOptions implements driver interface
func (d *Driver) GetDriverUpdateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}

	driverFlag.Options["enable-http-application-routing"] = &types.Flag{
		Type:  types.BoolType,
		Usage: `Enable the Kubernetes ingress with automatic public DNS name creation.`,
		Default: &types.Default{
			DefaultBool: false,
		},
	}
	driverFlag.Options["enable-monitoring"] = &types.Flag{
		Type:  types.BoolType,
		Usage: `Turn on Azure Log Analytics monitoring. Uses the Log Analytics "Default" workspace if it exists, else creates one. if using an existing workspace, specifies "log analytics workspace resource id".`,
		Default: &types.Default{
			DefaultBool: true,
		},
	}
	driverFlag.Options["log-analytics-workspace-resource-group"] = &types.Flag{
		Type:  types.StringType,
		Usage: `The resource group of an existing Azure Log Analytics Workspace to use for storing monitoring data. If not specified, uses the 'Cluster' resource group.`,
	}
	driverFlag.Options["log-analytics-workspace"] = &types.Flag{
		Type:  types.StringType,
		Usage: `The name of an existing Azure Log Analytics Workspace to use for storing monitoring data. If not specified, uses '{resource group}-{subscription id}-{location code}'.`,
	}

	driverFlag.Options["count"] = &types.Flag{
		Type:  types.IntType,
		Usage: "Number of machines (VMs) in the agent pool. Allowed values must be in the range of 1 to 100 (inclusive).",
		Default: &types.Default{
			DefaultInt: 1,
		},
	}

	driverFlag.Options["kubernetes-version"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Specify the version of Kubernetes.",
		Value: "1.11.5",
	}
	driverFlag.Options["tags"] = &types.Flag{
		Type:  types.StringSliceType,
		Usage: "Tags for Kubernetes cluster. For example, foo=bar.",
	}
	driverFlag.Options["client-id"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Azure client ID to use.",
	}
	driverFlag.Options["client-secret"] = &types.Flag{
		Type:     types.StringType,
		Password: true,
		Usage:    `Azure client secret associated with the "client id".`,
	}

	return &driverFlag, nil
}

// SetDriverOptions implements driver interface
func getStateFromOptions(driverOptions *types.DriverOptions) (state, error) {
	state := state{}

	state.SubscriptionID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "subscription-id", "subscriptionId").(string)
	state.ResourceGroup = options.GetValueFromDriverOptions(driverOptions, types.StringType, "resource-group", "resourceGroup").(string)
	state.Name = options.GetValueFromDriverOptions(driverOptions, types.StringType, "name").(string)

	state.AzureADClientAppID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "aad-client-app-id", "addClientAppId").(string)
	state.AzureADServerAppID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "aad-server-app-id", "addServerAppId").(string)
	state.AzureADServerAppSecret = options.GetValueFromDriverOptions(driverOptions, types.StringType, "aad-server-app-secret", "addServerAppSecret").(string)
	state.AzureADTenantID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "aad-tenant-id", "addTenantId").(string)

	state.AddonEnableHTTPApplicationRouting = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "enable-http-application-routing", "enableHttpApplicationRouting").(bool)
	state.AddonEnableMonitoring = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "enable-monitoring", "enableMonitoring").(bool)
	state.LogAnalyticsWorkspaceResourceGroup = options.GetValueFromDriverOptions(driverOptions, types.StringType, "log-analytics-workspace-resource-group", "logAnalyticsWorkspaceResourceGroup").(string)
	state.LogAnalyticsWorkspace = options.GetValueFromDriverOptions(driverOptions, types.StringType, "log-analytics-workspace", "logAnalyticsWorkspace").(string)

	state.AgentCount = options.GetValueFromDriverOptions(driverOptions, types.IntType, "count").(int64)
	state.AgentMaxPods = options.GetValueFromDriverOptions(driverOptions, types.IntType, "max-pods", "maxPods").(int64)
	state.AgentName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "agent-pool-name", "agentPoolName").(string)
	state.AgentOsdiskSizeGB = options.GetValueFromDriverOptions(driverOptions, types.IntType, "agent-osdisk-size", "agentOsdiskSize", "os-disk-size", "osDiskSizeGb").(int64)
	state.AgentVMSize = options.GetValueFromDriverOptions(driverOptions, types.StringType, "agent-vm-size", "agentVmSize").(string)
	state.VirtualNetworkResourceGroup = options.GetValueFromDriverOptions(driverOptions, types.StringType, "virtual-network-resource-group", "virtualNetworkResourceGroup").(string)
	state.VirtualNetwork = options.GetValueFromDriverOptions(driverOptions, types.StringType, "virtual-network", "virtualNetwork").(string)
	state.Subnet = options.GetValueFromDriverOptions(driverOptions, types.StringType, "subnet").(string)

	state.LinuxAdminUsername = options.GetValueFromDriverOptions(driverOptions, types.StringType, "admin-username", "adminUsername").(string)
	state.LinuxSSHPublicKeyContents = options.GetValueFromDriverOptions(driverOptions, types.StringType, "ssh-public-key-contents", "sshPublicKeyContents", "public-key-contents", "publicKeyContents").(string)
	state.LoadBalancerSku = options.GetValueFromDriverOptions(driverOptions, types.StringType, "load-balancer-sku", "loadBalancerSku").(string)

	state.NetworkDNSServiceIP = options.GetValueFromDriverOptions(driverOptions, types.StringType, "dns-service-ip", "dnsServiceIp").(string)
	state.NetworkDockerBridgeCIDR = options.GetValueFromDriverOptions(driverOptions, types.StringType, "docker-bridge-cidr", "dockerBridgeCidr").(string)
	state.NetworkPlugin = options.GetValueFromDriverOptions(driverOptions, types.StringType, "network-plugin", "networkPlugin").(string)
	state.NetworkPolicy = options.GetValueFromDriverOptions(driverOptions, types.StringType, "network-policy", "networkPolicy").(string)
	state.NetworkPodCIDR = options.GetValueFromDriverOptions(driverOptions, types.StringType, "pod-cidr", "podCidr").(string)
	state.NetworkServiceCIDR = options.GetValueFromDriverOptions(driverOptions, types.StringType, "service-cidr", "serviceCidr").(string)

	state.Location = options.GetValueFromDriverOptions(driverOptions, types.StringType, "location").(string)
	state.DNSPrefix = options.GetValueFromDriverOptions(driverOptions, types.StringType, "master-dns-prefix", "masterDnsPrefix").(string)
	state.KubernetesVersion = options.GetValueFromDriverOptions(driverOptions, types.StringType, "kubernetes-version", "kubernetesVersion").(string)
	state.Tags = make(map[string]string)
	tagValues := options.GetValueFromDriverOptions(driverOptions, types.StringSliceType, "tags").(*types.StringSlice)
	for _, part := range tagValues.Value {
		kv := strings.Split(part, "=")
		if len(kv) == 2 {
			state.Tags[kv[0]] = kv[1]
		}
	}

	state.BaseURL = options.GetValueFromDriverOptions(driverOptions, types.StringType, "base-url", "baseUrl").(string)
	state.AuthBaseURL = options.GetValueFromDriverOptions(driverOptions, types.StringType, "auth-base-url", "authBaseUrl").(string)
	state.ClientID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "client-id", "clientId").(string)
	state.ClientSecret = options.GetValueFromDriverOptions(driverOptions, types.StringType, "client-secret", "clientSecret").(string)
	state.TenantID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "tenant-id", "tenantId").(string)

	state.DisplayName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "display-name", "displayName").(string)

	return state, state.validate()
}

func (state state) validate() error {
	if state.SubscriptionID == "" {
		return fmt.Errorf(`"subscription id" is required`)
	}

	if state.ResourceGroup == "" {
		return fmt.Errorf(`"resource group" is required`)
	}

	if state.Name == "" {
		return fmt.Errorf(`"name" is required`)
	}

	if state.ClientID == "" {
		return fmt.Errorf(`"client id" is required`)
	}

	if state.ClientSecret == "" {
		return fmt.Errorf(`"client secret" is required`)
	}

	if state.Location == "" {
		return fmt.Errorf(`"location" is required`)
	}

	if state.LinuxSSHPublicKeyContents == "" {
		return fmt.Errorf(`"ssh public key contents" is required`)
	}
	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(state.LinuxSSHPublicKeyContents))
	if err != nil {
		if strings.Contains(state.LinuxSSHPublicKeyContents, "PRIVATE") {
			return fmt.Errorf("possible private key: %s", err)
		}
		return fmt.Errorf(`invalid ssh key: %s`, err)
	}

	return nil
}

func safeSlice(toSlice string, index int) string {
	size := len(toSlice)

	if index >= size {
		index = size - 1
	}

	return toSlice[:index]
}

func (state state) getDefaultDNSPrefix() string {
	namePart := safeSlice(state.Name, 10)
	groupPart := safeSlice(state.ResourceGroup, 16)
	subscriptionPart := safeSlice(state.SubscriptionID, 6)

	return fmt.Sprintf("%v-%v-%v", namePart, groupPart, subscriptionPart)
}

func newClientAuthorizer(state state) (autorest.Authorizer, error) {
	authBaseURL := state.AuthBaseURL
	if authBaseURL == "" {
		authBaseURL = azure.PublicCloud.ActiveDirectoryEndpoint
	}

	oauthConfig, err := adal.NewOAuthConfig(authBaseURL, state.TenantID)
	if err != nil {
		return nil, err
	}

	baseURL := state.BaseURL
	if baseURL == "" {
		baseURL = azure.PublicCloud.ResourceManagerEndpoint
	}

	spToken, err := adal.NewServicePrincipalToken(*oauthConfig, state.ClientID, state.ClientSecret, baseURL)
	if err != nil {
		return nil, err
	}

	authorizer := autorest.NewBearerAuthorizer(spToken)

	return authorizer, nil
}

func newClustersClient(authorizer autorest.Authorizer, state state) (*containerservice.ManagedClustersClient, error) {
	if authorizer == nil {
		newAuthorizer, err := newClientAuthorizer(state)
		if err != nil {
			return nil, err
		}

		authorizer = newAuthorizer
	}

	baseURL := state.BaseURL
	if baseURL == "" {
		baseURL = azure.PublicCloud.ResourceManagerEndpoint
	}

	client := containerservice.NewManagedClustersClientWithBaseURI(baseURL, state.SubscriptionID)
	client.Authorizer = authorizer

	return &client, nil
}

func newResourceGroupsClient(authorizer autorest.Authorizer, state state) (*resources.GroupsClient, error) {
	if authorizer == nil {
		newAuthorizer, err := newClientAuthorizer(state)
		if err != nil {
			return nil, err
		}

		authorizer = newAuthorizer
	}

	baseURL := state.BaseURL
	if baseURL == "" {
		baseURL = azure.PublicCloud.ResourceManagerEndpoint
	}

	client := resources.NewGroupsClientWithBaseURI(baseURL, state.SubscriptionID)
	client.Authorizer = authorizer

	return &client, nil
}

func newOperationInsightsWorkspaceClient(authorizer autorest.Authorizer, state state) (*operationalinsights.WorkspacesClient, error) {
	if authorizer == nil {
		newAuthorizer, err := newClientAuthorizer(state)
		if err != nil {
			return nil, err
		}

		authorizer = newAuthorizer
	}

	baseURL := state.BaseURL
	if baseURL == "" {
		baseURL = azure.PublicCloud.ResourceManagerEndpoint
	}

	client := operationalinsights.NewWorkspacesClientWithBaseURI(baseURL, state.SubscriptionID)
	client.Authorizer = authorizer

	return &client, nil
}

const failedStatus = "Failed"
const succeededStatus = "Succeeded"
const creatingStatus = "Creating"
const updatingStatus = "Updating"
const upgradingStatus = "Upgrading"

const pollInterval = 30

func (d *Driver) Create(ctx context.Context, options *types.DriverOptions, _ *types.ClusterInfo) (*types.ClusterInfo, error) {
	return d.createOrUpdate(ctx, options, true)
}

func (d *Driver) Update(ctx context.Context, info *types.ClusterInfo, options *types.DriverOptions) (*types.ClusterInfo, error) {
	return d.createOrUpdate(ctx, options, false)
}

func (d *Driver) createOrUpdate(ctx context.Context, options *types.DriverOptions, create bool) (*types.ClusterInfo, error) {
	driverState, err := getStateFromOptions(options)
	if err != nil {
		return nil, err
	}

	info := &types.ClusterInfo{}
	err = storeState(info, driverState)
	if err != nil {
		return info, err
	}

	azureAuthorizer, err := newClientAuthorizer(driverState)
	if err != nil {
		return info, err
	}

	clustersClient, err := newClustersClient(azureAuthorizer, driverState)
	if err != nil {
		return info, err
	}

	resourceGroupsClient, err := newResourceGroupsClient(azureAuthorizer, driverState)
	if err != nil {
		return info, err
	}

	workplacesClient, err := services.NewWorkplacesClient(azureAuthorizer, driverState.BaseURL, driverState.SubscriptionID)
	if err != nil {
		return info, err
	}

	masterDNSPrefix := driverState.DNSPrefix
	if masterDNSPrefix == "" {
		masterDNSPrefix = driverState.getDefaultDNSPrefix() + "-master"
	}

	tags := make(map[string]*string)
	for key, val := range driverState.Tags {
		if val != "" {
			tags[key] = to.StringPtr(val)
		}
	}
	displayName := driverState.DisplayName
	if displayName == "" {
		displayName = driverState.Name
	}
	tags["displayName"] = to.StringPtr(displayName)

	exists, err := d.resourceGroupExists(ctx, resourceGroupsClient, driverState.ResourceGroup)
	if err != nil {
		return info, err
	}

	if !exists {
		logrus.Infof("[azurekubernetesservice] resource group %v does not exist, creating", driverState.ResourceGroup)
		err = d.createResourceGroup(ctx, resourceGroupsClient, driverState)
		if err != nil {
			return info, err
		}
	}

	var aadProfile *containerservice.ManagedClusterAADProfile
	if driverState.hasAzureActiveDirectoryProfile() {
		aadProfile = &containerservice.ManagedClusterAADProfile{
			ClientAppID: to.StringPtr(driverState.AzureADClientAppID),
			ServerAppID: to.StringPtr(driverState.AzureADServerAppID),
		}

		if driverState.AzureADServerAppSecret != "" {
			aadProfile.ServerAppSecret = to.StringPtr(driverState.AzureADServerAppSecret)
		}

		if driverState.AzureADTenantID != "" {
			aadProfile.TenantID = to.StringPtr(driverState.AzureADTenantID)
		}
	}

	addonProfiles := map[string]*containerservice.ManagedClusterAddonProfile{
		"omsagent": {
			Enabled: to.BoolPtr(driverState.AddonEnableMonitoring),
		},
		"httpApplicationRouting": {
			Enabled: to.BoolPtr(driverState.AddonEnableHTTPApplicationRouting),
		},
	}
	if driverState.AddonEnableMonitoring {
		logAnalyticsWorkspaceResourceID, err := aks.CheckLogAnalyticsWorkspaceForMonitoring(ctx, workplacesClient,
			driverState.Location, driverState.ResourceGroup, driverState.LogAnalyticsWorkspaceResourceGroup, driverState.LogAnalyticsWorkspace)
		if err != nil {
			return info, err
		}

		if !strings.HasPrefix(logAnalyticsWorkspaceResourceID, "/") {
			logAnalyticsWorkspaceResourceID = "/" + logAnalyticsWorkspaceResourceID
		}
		logAnalyticsWorkspaceResourceID = strings.TrimSuffix(logAnalyticsWorkspaceResourceID, "/")

		addonProfiles["omsagent"].Config = map[string]*string{
			"logAnalyticsWorkspaceResourceID": to.StringPtr(logAnalyticsWorkspaceResourceID),
		}
	}
	if !driverState.hasHTTPApplicationRoutingSupport() {
		delete(addonProfiles, "httpApplicationRouting")
	}

	var vmNetSubnetID *string
	networkProfile := &containerservice.NetworkProfileType{}
	if driverState.hasCustomVirtualNetwork() {
		virtualNetworkResourceGroup := driverState.ResourceGroup

		// if virtual network resource group is set, use it, otherwise assume it is the same as the cluster
		if driverState.VirtualNetworkResourceGroup != "" {
			virtualNetworkResourceGroup = driverState.VirtualNetworkResourceGroup
		}

		vmNetSubnetID = to.StringPtr(fmt.Sprintf(
			"/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Network/virtualNetworks/%v/subnets/%v",
			driverState.SubscriptionID,
			virtualNetworkResourceGroup,
			driverState.VirtualNetwork,
			driverState.Subnet,
		))

		networkProfile.DNSServiceIP = to.StringPtr(driverState.NetworkDNSServiceIP)
		networkProfile.DockerBridgeCidr = to.StringPtr(driverState.NetworkDockerBridgeCIDR)
		networkProfile.ServiceCidr = to.StringPtr(driverState.NetworkServiceCIDR)

		if driverState.NetworkPlugin == "" {
			networkProfile.NetworkPlugin = containerservice.Azure
		} else {
			networkProfile.NetworkPlugin = containerservice.NetworkPlugin(driverState.NetworkPlugin)
		}

		// if network plugin is 'kubenet', set PodCIDR
		if networkProfile.NetworkPlugin == containerservice.Kubenet {
			networkProfile.PodCidr = to.StringPtr(driverState.NetworkPodCIDR)
		}

		if driverState.NetworkPolicy != "" {
			networkProfile.NetworkPolicy = containerservice.NetworkPolicy(driverState.NetworkPolicy)
		}
	}

	loadBalancerSku := containerservice.LoadBalancerSku(driverState.LoadBalancerSku)
	if create && containerservice.Standard == loadBalancerSku {
		networkProfile.LoadBalancerSku = loadBalancerSku
	}

	var agentPoolProfiles *[]containerservice.ManagedClusterAgentPoolProfile
	if driverState.hasAgentPoolProfile() {
		var countPointer *int32
		if driverState.AgentCount > 0 {
			countPointer = to.Int32Ptr(int32(driverState.AgentCount))
		} else {
			countPointer = to.Int32Ptr(1)
		}

		var maxPodsPointer *int32
		if driverState.AgentMaxPods > 0 {
			maxPodsPointer = to.Int32Ptr(int32(driverState.AgentMaxPods))
		} else {
			maxPodsPointer = to.Int32Ptr(110)
		}

		var osDiskSizeGBPointer *int32
		if driverState.AgentOsdiskSizeGB > 0 {
			osDiskSizeGBPointer = to.Int32Ptr(int32(driverState.AgentOsdiskSizeGB))
		}

		agentVMSize := containerservice.VMSizeTypesStandardD1V2
		if driverState.AgentVMSize != "" {
			agentVMSize = containerservice.VMSizeTypes(driverState.AgentVMSize)
		}

		agentPoolProfiles = &[]containerservice.ManagedClusterAgentPoolProfile{
			{
				Count:        countPointer,
				MaxPods:      maxPodsPointer,
				Name:         to.StringPtr(driverState.AgentName),
				OsDiskSizeGB: osDiskSizeGBPointer,
				OsType:       containerservice.Linux,
				VMSize:       agentVMSize,
				VnetSubnetID: vmNetSubnetID,
			},
		}
	}

	var linuxProfile *containerservice.LinuxProfile
	if driverState.hasLinuxProfile() {
		linuxProfile = &containerservice.LinuxProfile{
			AdminUsername: to.StringPtr(driverState.LinuxAdminUsername),
			SSH: &containerservice.SSHConfiguration{
				PublicKeys: &[]containerservice.SSHPublicKey{
					{
						KeyData: to.StringPtr(driverState.LinuxSSHPublicKeyContents),
					},
				},
			},
		}
	}

	managedCluster := containerservice.ManagedCluster{
		Location: to.StringPtr(driverState.Location),
		Tags:     tags,
		ManagedClusterProperties: &containerservice.ManagedClusterProperties{
			KubernetesVersion: to.StringPtr(driverState.KubernetesVersion),
			DNSPrefix:         to.StringPtr(masterDNSPrefix),
			AadProfile:        aadProfile,
			AddonProfiles:     addonProfiles,
			AgentPoolProfiles: agentPoolProfiles,
			LinuxProfile:      linuxProfile,
			NetworkProfile:    networkProfile,
			ServicePrincipalProfile: &containerservice.ManagedClusterServicePrincipalProfile{
				ClientID: to.StringPtr(driverState.ClientID),
				Secret:   to.StringPtr(driverState.ClientSecret),
			},
		},
	}

	if create {
		managedCluster.ManagedClusterProperties.EnableRBAC = to.BoolPtr(true)
	}

	logClusterConfig(managedCluster)
	_, err = clustersClient.CreateOrUpdate(ctx, driverState.ResourceGroup, driverState.Name, managedCluster)
	if err != nil {
		return info, err
	}

	logrus.Infof("[azurekubernetesservice] Request submitted, waiting for cluster [%s] to finish creating", driverState.Name)

	failedCount := 0

	for {
		result, err := clustersClient.Get(ctx, driverState.ResourceGroup, driverState.Name)
		if err != nil {
			return info, err
		}

		state := *result.ProvisioningState

		if state == failedStatus {
			if failedCount > 3 {
				logrus.Errorf("cluster recovery failed, retries depleted")
				return info, fmt.Errorf("cluster create has completed with status of 'Failed'")
			}

			failedCount = failedCount + 1
			logrus.Infof("[azurekubernetesservice] cluster [%s] marked as failed but waiting for recovery: retries left %v", driverState.Name, 3-failedCount)
			time.Sleep(pollInterval * time.Second)
		}

		if state == succeededStatus {
			logrus.Infof("[azurekubernetesservice] Cluster [%s] provisioned successfully", driverState.Name)
			info := &types.ClusterInfo{}
			err := storeState(info, driverState)

			return info, err
		}

		if state != creatingStatus && state != updatingStatus && state != upgradingStatus {
			logrus.Errorf("Azure failed to provision cluster with state: %v", state)
			return info, fmt.Errorf("failed to provision Azure cluster")
		}

		logrus.Infof("[azurekubernetesservice] Cluster [%s] has not yet completed provisioning, waiting another %v seconds", driverState.Name, pollInterval)

		time.Sleep(pollInterval * time.Second)
	}
}

func (state state) hasCustomVirtualNetwork() bool {
	return state.VirtualNetwork != "" && state.Subnet != ""
}

func (state state) hasAzureActiveDirectoryProfile() bool {
	return state.AzureADClientAppID != "" && state.AzureADServerAppID != "" && state.AzureADServerAppSecret != ""
}

func (state state) hasAgentPoolProfile() bool {
	return state.AgentName != ""
}

func (state state) hasLinuxProfile() bool {
	return state.LinuxAdminUsername != "" && (state.LinuxSSHPublicKeyContents != "")
}

func (state state) hasHTTPApplicationRoutingSupport() bool {
	// HttpApplicationRouting is not supported in azure china cloud
	return !strings.HasPrefix(state.Location, "china")
}

func (d *Driver) resourceGroupExists(ctx context.Context, client *resources.GroupsClient, groupName string) (bool, error) {
	resp, err := client.CheckExistence(ctx, groupName)
	if err != nil {
		return false, fmt.Errorf("error getting Resource Group '%s': %v", groupName, err)
	}

	return resp.StatusCode == 204, nil
}

func (d *Driver) createResourceGroup(ctx context.Context, client *resources.GroupsClient, state state) error {
	resourceGroup, location := state.ResourceGroup, state.Location

	_, err := client.CreateOrUpdate(ctx, resourceGroup, resources.Group{
		Name:     to.StringPtr(resourceGroup),
		Location: to.StringPtr(location),
	})
	if err != nil {
		return fmt.Errorf("error creating Resource Group '%s': %v", resourceGroup, err)
	}

	return nil
}

func storeState(info *types.ClusterInfo, state state) error {
	data, err := json.Marshal(state)

	if err != nil {
		return err
	}

	if info.Metadata == nil {
		info.Metadata = map[string]string{}
	}

	info.Metadata["state"] = string(data)
	info.Metadata["resource-group"] = state.ResourceGroup
	info.Metadata["location"] = state.Location

	return nil
}

func getState(info *types.ClusterInfo) (state, error) {
	state := state{}

	err := json.Unmarshal([]byte(info.Metadata["state"]), &state)

	if err != nil {
		logrus.Errorf("Error encountered while marshalling state: %v", err)
	}

	return state, err
}

func (d *Driver) GetVersion(ctx context.Context, info *types.ClusterInfo) (*types.KubernetesVersion, error) {
	state, err := getState(info)

	if err != nil {
		return nil, err
	}

	client, err := newClustersClient(nil, state)

	if err != nil {
		return nil, err
	}

	cluster, err := client.Get(context.Background(), state.ResourceGroup, state.Name)

	if err != nil {
		return nil, fmt.Errorf("error getting cluster info: %v", err)
	}

	return &types.KubernetesVersion{Version: *cluster.KubernetesVersion}, nil
}

func (d *Driver) SetVersion(ctx context.Context, info *types.ClusterInfo, version *types.KubernetesVersion) error {
	state, err := getState(info)

	if err != nil {
		return err
	}

	client, err := newClustersClient(nil, state)

	if err != nil {
		return err
	}

	cluster, err := client.Get(context.Background(), state.ResourceGroup, state.Name)

	if err != nil {
		return fmt.Errorf("error getting cluster info: %v", err)
	}

	cluster.KubernetesVersion = to.StringPtr(version.Version)

	_, err = client.CreateOrUpdate(context.Background(), state.ResourceGroup, state.Name, cluster)

	if err != nil {
		return fmt.Errorf("error updating kubernetes version: %v", err)
	}

	return nil
}

func (d *Driver) GetClusterSize(ctx context.Context, info *types.ClusterInfo) (*types.NodeCount, error) {
	state, err := getState(info)

	if err != nil {
		return nil, err
	}

	client, err := newClustersClient(nil, state)

	if err != nil {
		return nil, err
	}

	result, err := client.Get(context.Background(), state.ResourceGroup, state.Name)

	if err != nil {
		return nil, fmt.Errorf("error getting cluster info: %v", err)
	}

	return &types.NodeCount{Count: int64(*(*result.AgentPoolProfiles)[0].Count)}, nil
}

func (d *Driver) SetClusterSize(ctx context.Context, info *types.ClusterInfo, size *types.NodeCount) error {
	state, err := getState(info)

	if err != nil {
		return err
	}

	client, err := newClustersClient(nil, state)

	if err != nil {
		return err
	}

	cluster, err := client.Get(context.Background(), state.ResourceGroup, state.Name)

	if err != nil {
		return fmt.Errorf("error getting cluster info: %v", err)
	}

	// mutate struct
	(*cluster.ManagedClusterProperties.AgentPoolProfiles)[0].Count = to.Int32Ptr(int32(size.Count))

	// PUT same data
	_, err = client.CreateOrUpdate(context.Background(), state.ResourceGroup, state.Name, cluster)

	if err != nil {
		return fmt.Errorf("error updating cluster size: %v", err)
	}

	return nil
}

// KubeConfig struct for marshalling config files
// shouldn't have to reimplement this but kubernetes' model won't serialize correctly for some reason
type KubeConfig struct {
	APIVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	Clusters   []Cluster `yaml:"clusters"`
	Contexts   []Context `yaml:"contexts"`
	Users      []User    `yaml:"users"`
}

type Cluster struct {
	Name        string      `yaml:"name"`
	ClusterInfo ClusterInfo `yaml:"cluster"`
}

type ClusterInfo struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
}

type Context struct {
	ContextInfo ContextInfo `yaml:"context"`
	Name        string      `yaml:"name"`
}

type ContextInfo struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

type User struct {
	UserInfo UserInfo `yaml:"user"`
	Name     string   `yaml:"name"`
}

type UserInfo struct {
	ClientCertificateData string `yaml:"client-certificate-data"`
	ClientKeyData         string `yaml:"client-key-data"`
	Token                 string `yaml:"token"`
}

const retries = 5

func (d *Driver) PostCheck(ctx context.Context, info *types.ClusterInfo) (*types.ClusterInfo, error) {
	logrus.Info("[azurekubernetesservice] starting post-check")

	clientset, err := getClientset(info)
	if err != nil {
		return nil, err
	}

	failureCount := 0

	for {
		info.ServiceAccountToken, err = util.GenerateServiceAccountToken(clientset)

		if err == nil {
			logrus.Info("[azurekubernetesservice] service account token generated successfully")
			break
		} else {
			if failureCount < retries {
				logrus.Infof("[azurekubernetesservice] service account token generation failed, retries left: %v", retries-failureCount)
				failureCount = failureCount + 1

				time.Sleep(pollInterval * time.Second)
			} else {
				logrus.Error("retries exceeded, failing post-check")
				return nil, err
			}
		}
	}

	logrus.Info("[azurekubernetesservice] post-check completed successfully")

	return info, nil
}

func getClientset(info *types.ClusterInfo) (*kubernetes.Clientset, error) {
	state, err := getState(info)

	if err != nil {
		return nil, err
	}

	client, err := newClustersClient(nil, state)

	if err != nil {
		return nil, err
	}

	result, err := client.GetAccessProfile(context.Background(), state.ResourceGroup, state.Name, "clusterUser")

	if err != nil {
		return nil, err
	}

	clusterConfig := KubeConfig{}
	err = yaml.Unmarshal(*result.KubeConfig, &clusterConfig)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal kubeconfig: %v", err)
	}

	singleCluster := clusterConfig.Clusters[0]
	singleUser := clusterConfig.Users[0]

	info.Version = clusterConfig.APIVersion
	info.Endpoint = singleCluster.ClusterInfo.Server
	info.Password = singleUser.UserInfo.Token
	info.RootCaCertificate = singleCluster.ClusterInfo.CertificateAuthorityData
	info.ClientCertificate = singleUser.UserInfo.ClientCertificateData
	info.ClientKey = singleUser.UserInfo.ClientKeyData

	capem, err := base64.StdEncoding.DecodeString(singleCluster.ClusterInfo.CertificateAuthorityData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode CA: %v", err)
	}

	key, err := base64.StdEncoding.DecodeString(singleUser.UserInfo.ClientKeyData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode client key: %v", err)
	}

	cert, err := base64.StdEncoding.DecodeString(singleUser.UserInfo.ClientCertificateData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode client cert: %v", err)
	}

	host := singleCluster.ClusterInfo.Server
	if !strings.HasPrefix(host, "https://") {
		host = fmt.Sprintf("https://%s", host)
	}

	config := &rest.Config{
		Host: host,
		TLSClientConfig: rest.TLSClientConfig{
			CAData:   capem,
			KeyData:  key,
			CertData: cert,
		},
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset: %v", err)
	}

	return clientset, nil
}

func (d *Driver) Remove(ctx context.Context, info *types.ClusterInfo) error {
	state, err := getState(info)

	if err != nil {
		return err
	}

	client, err := newClustersClient(nil, state)

	if err != nil {
		return err
	}

	_, err = client.Delete(context.Background(), state.ResourceGroup, state.Name)

	if err != nil {
		return err
	}

	logrus.Infof("[azurekubernetesservice] Cluster [%v] removed successfully", state.Name)

	return nil
}

func (d *Driver) GetCapabilities(ctx context.Context) (*types.Capabilities, error) {
	return &d.driverCapabilities, nil
}

func (d *Driver) RemoveLegacyServiceAccount(ctx context.Context, info *types.ClusterInfo) error {
	clientset, err := getClientset(info)
	if err != nil {
		return err
	}

	return util.DeleteLegacyServiceAccountAndRoleBinding(clientset)
}

func logClusterConfig(config containerservice.ManagedCluster) {
	if logrus.GetLevel() == logrus.DebugLevel {
		out, err := json.Marshal(config)
		if err != nil {
			logrus.Error("Error marshalling config for logging")
			return
		}
		output := string(out)
		output = redactionRegex.ReplaceAllString(output, "$1: [REDACTED]")
		logrus.Debugf("Sending cluster config to AKS: %v", output)
	}
}

func (d *Driver) ETCDSave(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) error {
	return fmt.Errorf("ETCD backup operations are not implemented")
}

func (d *Driver) ETCDRestore(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) (*types.ClusterInfo, error) {
	return nil, fmt.Errorf("ETCD backup operations are not implemented")
}

func (d *Driver) ETCDRemoveSnapshot(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) error {
	return fmt.Errorf("ETCD backup operations are not implemented")
}

func (d *Driver) GetK8SCapabilities(ctx context.Context, _ *types.DriverOptions) (*types.K8SCapabilities, error) {
	return &types.K8SCapabilities{
		L4LoadBalancer: &types.LoadBalancerCapabilities{
			Enabled:              true,
			Provider:             "Azure L4 LB",
			ProtocolsSupported:   []string{"TCP", "UDP"},
			HealthCheckSupported: true,
		},
	}, nil
}
