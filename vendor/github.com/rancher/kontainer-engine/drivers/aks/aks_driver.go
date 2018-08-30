package aks

import (
	"strings"

	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2018-03-31/containerservice"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2017-05-10/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"github.com/rancher/kontainer-engine/drivers/options"
	"github.com/rancher/kontainer-engine/drivers/util"
	"github.com/rancher/kontainer-engine/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Driver struct {
	driverCapabilities types.Capabilities
}

type state struct {
	// Path to the public key to use for SSH into cluster
	SSHPublicKeyPath string `json:"sshPublicKeyPath,omitempty"`

	v3.AzureKubernetesServiceConfig

	// Cluster Name
	Name string

	// The name that is displayed to the user on the Rancher UI
	DisplayName string

	// Cluster info
	ClusterInfo types.ClusterInfo
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
	driverFlag.Options["display-name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The name that is displayed to the user on the Rancher UI",
	}
	driverFlag.Options["subscription-id"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Subscription credentials which uniquely identify Microsoft Azure subscription",
	}
	driverFlag.Options["resource-group"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The name of the resource group",
	}
	driverFlag.Options["location"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Resource location",
		Value: "eastus",
	}
	driverFlag.Options["tags"] = &types.Flag{
		Type:  types.StringSliceType,
		Usage: "Resource tags. For example, foo=bar",
	}
	driverFlag.Options["node-count"] = &types.Flag{
		Type:  types.IntType,
		Usage: "Number of agents (VMs) to host docker containers. Allowed values must be in the range of 1 to 100 (inclusive)",
		Value: "1",
	}
	driverFlag.Options["node-dns-prefix"] = &types.Flag{
		Type:  types.StringType,
		Usage: "DNS prefix to be used to create the FQDN for the agent pool",
	}
	driverFlag.Options["node-pool-name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Name for the agent pool",
		Value: "agentpool0",
	}
	driverFlag.Options["os-disk-size"] = &types.Flag{
		Type:  types.StringType,
		Usage: "OS Disk Size in GB to be used to specify the disk size for every machine in this master/agent pool. If you specify 0, it will apply the default osDisk size according to the vmSize specified.",
	}
	driverFlag.Options["node-vm-size"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Size of agent VMs",
		Value: "Standard_D1_v2",
	}
	driverFlag.Options["kubernetes-version"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Version of Kubernetes specified when creating the managed cluster",
		Value: "1.7.9",
	}
	driverFlag.Options["public-key"] = &types.Flag{
		Type:  types.StringType,
		Usage: "SSH public key to use for the cluster",
	}
	driverFlag.Options["master-dns-prefix"] = &types.Flag{
		Type:  types.StringType,
		Usage: "DNS prefix to use for the master",
	}
	driverFlag.Options["admin-username"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Admin username to use for the cluster",
		Value: "azureuser",
	}
	driverFlag.Options["base-url"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Different base API url to use",
		Value: containerservice.DefaultBaseURI,
	}
	driverFlag.Options["client-id"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Azure client id to use",
	}
	driverFlag.Options["client-secret"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Client secret associated with the client-id",
	}
	driverFlag.Options["tenant-id"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Azure tenant id to use",
	}
	driverFlag.Options["virtual-network"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Azure virtual network to use",
	}
	driverFlag.Options["subnet"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Azure subnet to use",
	}
	driverFlag.Options["virtual-network-resource-group"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The resource group that the virtual network is in",
	}
	driverFlag.Options["service-cidr"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The IP range to assign service cluster IPs.",
		Value: "10.0.0.0/16",
	}
	driverFlag.Options["dns-service-ip"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The IP address to asign to the Kubernetes DNS service.  It must be within the Service CIDR range.",
		Value: "10.0.0.10",
	}
	driverFlag.Options["docker-bridge-cidr"] = &types.Flag{
		Type:  types.StringType,
		Usage: "A IP range for the Docker bridge network.  It must not overlap the subnet address range of the service CIDR.",
		Value: "172.17.0.1/16",
	}

	return &driverFlag, nil
}

// GetDriverUpdateOptions implements driver interface
func (d *Driver) GetDriverUpdateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}
	driverFlag.Options["node-count"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Number of agents (VMs) to host docker containers. Allowed values must be in the range of 1 to 100 (inclusive)",
		Value: "1",
	}
	driverFlag.Options["kubernetes-version"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Version of Kubernetes specified when creating the managed cluster",
	}
	return &driverFlag, nil
}

// SetDriverOptions implements driver interface
func getStateFromOptions(driverOptions *types.DriverOptions) (state, error) {
	state := state{}
	state.Name = options.GetValueFromDriverOptions(driverOptions, types.StringType, "name").(string)
	state.DisplayName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "display-name", "displayName").(string)
	state.AgentDNSPrefix = options.GetValueFromDriverOptions(driverOptions, types.StringType, "node-dns-prefix", "agentDnsPrefix").(string)
	state.AgentVMSize = options.GetValueFromDriverOptions(driverOptions, types.StringType, "node-vm-size", "agentVmSize").(string)
	state.Count = options.GetValueFromDriverOptions(driverOptions, types.IntType, "node-count", "count").(int64)
	state.KubernetesVersion = options.GetValueFromDriverOptions(driverOptions, types.StringType, "kubernetes-version", "kubernetesVersion").(string)
	state.Location = options.GetValueFromDriverOptions(driverOptions, types.StringType, "location").(string)
	state.OsDiskSizeGB = options.GetValueFromDriverOptions(driverOptions, types.IntType, "os-disk-size", "osDiskSizeGb").(int64)
	state.SubscriptionID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "subscription-id", "subscriptionId").(string)
	state.ResourceGroup = options.GetValueFromDriverOptions(driverOptions, types.StringType, "resource-group", "resourceGroup").(string)
	state.AgentPoolName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "node-pool-name", "agentPoolName").(string)
	state.MasterDNSPrefix = options.GetValueFromDriverOptions(driverOptions, types.StringType, "master-dns-prefix", "masterDnsPrefix").(string)
	state.SSHPublicKeyPath = options.GetValueFromDriverOptions(driverOptions, types.StringType, "public-key").(string)
	state.SSHPublicKeyContents = options.GetValueFromDriverOptions(driverOptions, types.StringType, "sshPublicKeyContents").(string)
	state.AdminUsername = options.GetValueFromDriverOptions(driverOptions, types.StringType, "admin-username", "adminUsername").(string)
	state.BaseURL = options.GetValueFromDriverOptions(driverOptions, types.StringType, "base-url").(string)
	state.ClientID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "client-id", "clientId").(string)
	state.TenantID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "tenant-id", "tenantId").(string)
	state.ClientSecret = options.GetValueFromDriverOptions(driverOptions, types.StringType, "client-secret", "clientSecret").(string)
	state.VirtualNetwork = options.GetValueFromDriverOptions(driverOptions, types.StringType, "virtualNetwork", "virtual-network").(string)
	state.Subnet = options.GetValueFromDriverOptions(driverOptions, types.StringType, "subnet").(string)
	state.VirtualNetworkResourceGroup = options.GetValueFromDriverOptions(driverOptions, types.StringType, "virtualNetworkResourceGroup", "virtual-network-resource-group").(string)
	state.ServiceCIDR = options.GetValueFromDriverOptions(driverOptions, types.StringType, "serviceCidr", "service-cidr").(string)
	state.DNSServiceIP = options.GetValueFromDriverOptions(driverOptions, types.StringType, "dnsServiceIp", "dns-service-ip").(string)
	state.DockerBridgeCIDR = options.GetValueFromDriverOptions(driverOptions, types.StringType, "dockerBridgeCidr", "docker-bridge-cidr").(string)
	tagValues := options.GetValueFromDriverOptions(driverOptions, types.StringSliceType).(*types.StringSlice)
	for _, part := range tagValues.Value {
		kv := strings.Split(part, "=")
		if len(kv) == 2 {
			state.Tag[kv[0]] = kv[1]
		}
	}
	return state, state.validate()
}

func (state *state) validate() error {
	if state.Name == "" {
		return fmt.Errorf("cluster name is required")
	}

	if state.ResourceGroup == "" {
		return fmt.Errorf("resource group is required")
	}

	if state.SSHPublicKeyPath == "" && state.SSHPublicKeyContents == "" {
		return fmt.Errorf("path to ssh public key or public key contents is required")
	}

	if state.ClientID == "" {
		return fmt.Errorf("client id is required")
	}

	if state.ClientSecret == "" {
		return fmt.Errorf("client secret is required")
	}

	if state.SubscriptionID == "" {
		return fmt.Errorf("subscription id is required")
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

func (state *state) getDefaultDNSPrefix() string {
	namePart := safeSlice(state.Name, 10)
	groupPart := safeSlice(state.ResourceGroup, 16)
	subscriptionPart := safeSlice(state.SubscriptionID, 6)

	return fmt.Sprintf("%v-%v-%v", namePart, groupPart, subscriptionPart)
}

func newAzureClient(state state) (*containerservice.ManagedClustersClient, error) {
	oauthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, state.TenantID)
	if err != nil {
		return nil, err
	}

	spToken, err := adal.NewServicePrincipalToken(*oauthConfig, state.ClientID, state.ClientSecret, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}

	authorizer := autorest.NewBearerAuthorizer(spToken)

	baseURL := state.BaseURL
	if baseURL == "" {
		baseURL = containerservice.DefaultBaseURI
	}

	client := containerservice.NewManagedClustersClientWithBaseURI(baseURL, state.SubscriptionID)
	client.Authorizer = authorizer

	return &client, nil
}

func newResourcesClient(state state) (*resources.GroupsClient, error) {
	oauthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, state.TenantID)
	if err != nil {
		return nil, err
	}

	spToken, err := adal.NewServicePrincipalToken(*oauthConfig, state.ClientID, state.ClientSecret, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}

	authorizer := autorest.NewBearerAuthorizer(spToken)

	baseURL := state.BaseURL
	if baseURL == "" {
		baseURL = containerservice.DefaultBaseURI
	}

	client := resources.NewGroupsClientWithBaseURI(baseURL, state.SubscriptionID)
	client.Authorizer = authorizer

	return &client, nil
}

const failedStatus = "Failed"
const succeededStatus = "Succeeded"
const creatingStatus = "Creating"
const updatingStatus = "Updating"

const pollInterval = 30

func (d *Driver) Create(ctx context.Context, options *types.DriverOptions, _ *types.ClusterInfo) (*types.ClusterInfo, error) {
	return d.createOrUpdate(ctx, options)
}

func (d *Driver) Update(ctx context.Context, info *types.ClusterInfo, options *types.DriverOptions) (*types.ClusterInfo, error) {
	return d.createOrUpdate(ctx, options)
}

func (d *Driver) createOrUpdate(ctx context.Context, options *types.DriverOptions) (*types.ClusterInfo, error) {
	driverState, err := getStateFromOptions(options)
	if err != nil {
		return nil, err
	}

	clustersClient, err := newAzureClient(driverState)
	if err != nil {
		return nil, err
	}

	resourcesClient, err := newResourcesClient(driverState)
	if err != nil {
		return nil, err
	}

	masterDNSPrefix := driverState.MasterDNSPrefix
	if masterDNSPrefix == "" {
		masterDNSPrefix = driverState.getDefaultDNSPrefix() + "-master"
	}

	agentDNSPrefix := driverState.AgentDNSPrefix
	if agentDNSPrefix == "" {
		agentDNSPrefix = driverState.getDefaultDNSPrefix() + "-agent"
	}

	var publicKey []byte

	if driverState.SSHPublicKeyContents == "" {
		publicKey, err = ioutil.ReadFile(driverState.SSHPublicKeyPath)
	} else {
		publicKey = []byte(driverState.SSHPublicKeyContents)
	}

	if err != nil {
		return nil, err
	}

	publicKeyContents := string(publicKey)
	tags := make(map[string]*string)

	displayName := driverState.DisplayName
	if displayName == "" {
		displayName = driverState.Name
	}
	tags["displayName"] = to.StringPtr(displayName)

	exists, err := d.resourceGroupExists(ctx, resourcesClient, driverState.ResourceGroup)
	if err != nil {
		return nil, err
	}

	if !exists {
		logrus.Infof("resource group %v does not exist, creating", driverState.ResourceGroup)
		err = d.createResourceGroup(ctx, resourcesClient, driverState)
		if err != nil {
			return nil, err
		}
	}

	var vmNetSubnetID *string
	if d.hasCustomVirtualNetwork(driverState) {
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
	}

	managedCluster := containerservice.ManagedCluster{
		Location: to.StringPtr(driverState.Location),
		Tags:     tags,
		ManagedClusterProperties: &containerservice.ManagedClusterProperties{
			KubernetesVersion: to.StringPtr(driverState.KubernetesVersion),
			DNSPrefix:         to.StringPtr(masterDNSPrefix),
			LinuxProfile: &containerservice.LinuxProfile{
				AdminUsername: to.StringPtr(driverState.AdminUsername),
				SSH: &containerservice.SSHConfiguration{
					PublicKeys: &[]containerservice.SSHPublicKey{
						{
							KeyData: to.StringPtr(publicKeyContents),
						},
					},
				},
			},
			AgentPoolProfiles: &[]containerservice.ManagedClusterAgentPoolProfile{
				{
					DNSPrefix:    to.StringPtr(agentDNSPrefix),
					Name:         to.StringPtr(driverState.AgentPoolName),
					VMSize:       containerservice.VMSizeTypes(driverState.AgentVMSize),
					Count:        to.Int32Ptr(int32(driverState.Count)),
					VnetSubnetID: vmNetSubnetID,
				},
			},
			ServicePrincipalProfile: &containerservice.ServicePrincipalProfile{
				ClientID: to.StringPtr(driverState.ClientID),
				Secret:   to.StringPtr(driverState.ClientSecret),
			},
		},
	}

	if d.hasCustomVirtualNetwork(driverState) {
		managedCluster.NetworkProfile = &containerservice.NetworkProfile{
			DNSServiceIP:     to.StringPtr(driverState.DNSServiceIP),
			DockerBridgeCidr: to.StringPtr(driverState.DockerBridgeCIDR),
			ServiceCidr:      to.StringPtr(driverState.ServiceCIDR),
			NetworkPlugin:    containerservice.Azure,
		}
	}

	_, err = clustersClient.CreateOrUpdate(ctx, driverState.ResourceGroup, driverState.Name, managedCluster)
	if err != nil {
		return nil, err
	}

	logrus.Info("Request submitted, waiting for cluster to finish creating")

	failedCount := 0

	for {
		result, err := clustersClient.Get(ctx, driverState.ResourceGroup, driverState.Name)
		if err != nil {
			return nil, err
		}

		state := *result.ProvisioningState

		if state == failedStatus {
			if failedCount > 3 {
				logrus.Errorf("cluster recovery failed, retries depleted")
				return nil, fmt.Errorf("cluster create has completed with status of 'Failed'")
			}

			failedCount = failedCount + 1
			logrus.Infof("cluster marked as failed but waiting for recovery: retries left %v", 3-failedCount)
			time.Sleep(pollInterval * time.Second)
		}

		if state == succeededStatus {
			logrus.Info("Cluster provisioned successfully")
			info := &types.ClusterInfo{}
			err := storeState(info, driverState)

			return info, err
		}

		if state != creatingStatus && state != updatingStatus {
			logrus.Errorf("Azure failed to provision cluster with state: %v", state)
			return nil, errors.New("Azure failed to provision cluster")
		}

		logrus.Infof("Cluster has not yet completed provisioning, waiting another %v seconds", pollInterval)

		time.Sleep(pollInterval * time.Second)
	}
}

func (d *Driver) hasCustomVirtualNetwork(driverState state) bool {
	return driverState.VirtualNetwork != "" && driverState.Subnet != ""
}

func (d *Driver) resourceGroupExists(ctx context.Context, client *resources.GroupsClient, groupName string) (bool, error) {
	resp, err := client.CheckExistence(ctx, groupName)
	if err != nil {
		return false, fmt.Errorf("error getting resource group %v: %v", groupName, err)
	}

	return resp.StatusCode == 204, nil
}

func (d *Driver) createResourceGroup(ctx context.Context, client *resources.GroupsClient, state state) error {
	_, err := client.CreateOrUpdate(ctx, state.ResourceGroup, resources.Group{
		Name:     to.StringPtr(state.ResourceGroup),
		Location: to.StringPtr(state.Location),
	})
	if err != nil {
		return fmt.Errorf("error creating resource group %v: %v", state.ResourceGroup, err)
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

	client, err := newAzureClient(state)

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

	client, err := newAzureClient(state)

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

	client, err := newAzureClient(state)

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

	client, err := newAzureClient(state)

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
	logrus.Info("starting post-check")

	state, err := getState(info)

	if err != nil {
		return nil, err
	}

	client, err := newAzureClient(state)

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

	failureCount := 0

	for {
		info.ServiceAccountToken, err = util.GenerateServiceAccountToken(clientset)

		if err == nil {
			logrus.Info("service account token generated successfully")
			break
		} else {
			if failureCount < retries {
				logrus.Infof("service account token generation failed, retries left: %v", retries-failureCount)
				failureCount = failureCount + 1

				time.Sleep(pollInterval * time.Second)
			} else {
				logrus.Error("retries exceeded, failing post-check")
				return nil, err
			}
		}
	}

	logrus.Info("post-check completed successfully")

	return info, nil
}

func (d *Driver) Remove(ctx context.Context, info *types.ClusterInfo) error {
	state, err := getState(info)

	if err != nil {
		return err
	}

	client, err := newAzureClient(state)

	if err != nil {
		return err
	}

	_, err = client.Delete(context.Background(), state.ResourceGroup, state.Name)

	if err != nil {
		return err
	}

	logrus.Infof("Cluster %v removed successfully", state.Name)

	return nil
}

func (d *Driver) GetCapabilities(ctx context.Context) (*types.Capabilities, error) {
	return &d.driverCapabilities, nil
}
