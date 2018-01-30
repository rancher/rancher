package aks

import (
	"strings"

	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2017-08-31/containerservice"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/Azure/go-autorest/autorest/utils"
	"github.com/rancher/kontainer-engine/types"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Driver struct {
	driverCapabilities types.Capabilities
}

type state struct {
	// Subscription credentials which uniquely identify Microsoft Azure subscription. The subscription ID forms part of the URI for every service call.
	SubscriptionID string
	// The name of the resource group.
	ResourceGroup string
	// The name of the managed cluster resource.
	Name string
	// Resource location
	Location string
	// Resource tags
	Tag map[string]string
	// Number of agents (VMs) to host docker containers. Allowed values must be in the range of 1 to 100 (inclusive). The default value is 1.
	Count int64
	// DNS prefix to be used to create the FQDN for the agent pool.
	AgentDNSPrefix string
	// FDQN for the agent pool
	AgentPoolName string
	// OS Disk Size in GB to be used to specify the disk size for every machine in this master/agent pool. If you specify 0, it will apply the default osDisk size according to the vmSize specified.
	OsDiskSizeGB int64
	// Size of agent VMs
	AgentVMSize string
	// Version of Kubernetes specified when creating the managed cluster
	KubernetesVersion string
	// Path to the public key to use for SSH into cluster
	SSHPublicKeyPath string
	// Kubernetes Master DNS prefix (must be unique within Azure)
	MasterDNSPrefix string
	// Kubernetes admin username
	AdminUsername string
	// Different Base URL if required, usually needed for testing purposes
	BaseURL string
	// Azure Client ID to use
	ClientID string
	// Secret associated with the Client ID
	ClientSecret string

	// Cluster info
	ClusterInfo types.ClusterInfo
}

func NewDriver() *Driver {
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
		Type:  types.StringType,
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
	state.Name = getValueFromDriverOptions(driverOptions, types.StringType, "name").(string)
	state.AgentDNSPrefix = getValueFromDriverOptions(driverOptions, types.StringType, "node-dns-prefix").(string)
	state.AgentVMSize = getValueFromDriverOptions(driverOptions, types.StringType, "node-vm-size").(string)
	state.Count = getValueFromDriverOptions(driverOptions, types.IntType, "node-count").(int64)
	state.KubernetesVersion = getValueFromDriverOptions(driverOptions, types.StringType, "kubernetes-version").(string)
	state.Location = getValueFromDriverOptions(driverOptions, types.StringType, "location").(string)
	state.OsDiskSizeGB = getValueFromDriverOptions(driverOptions, types.IntType, "os-disk-size").(int64)
	state.SubscriptionID = getValueFromDriverOptions(driverOptions, types.StringType, "subscription-id").(string)
	state.ResourceGroup = getValueFromDriverOptions(driverOptions, types.StringType, "resource-group").(string)
	state.AgentPoolName = getValueFromDriverOptions(driverOptions, types.StringType, "node-pool-name").(string)
	state.MasterDNSPrefix = getValueFromDriverOptions(driverOptions, types.StringType, "master-dns-prefix").(string)
	state.SSHPublicKeyPath = getValueFromDriverOptions(driverOptions, types.StringType, "public-key").(string)
	state.AdminUsername = getValueFromDriverOptions(driverOptions, types.StringType, "admin-username").(string)
	state.BaseURL = getValueFromDriverOptions(driverOptions, types.StringType, "base-url").(string)
	state.ClientID = getValueFromDriverOptions(driverOptions, types.StringType, "client-id").(string)
	state.ClientSecret = getValueFromDriverOptions(driverOptions, types.StringType, "client-secret").(string)
	tagValues := getValueFromDriverOptions(driverOptions, types.StringSliceType).(*types.StringSlice)
	for _, part := range tagValues.Value {
		kv := strings.Split(part, "=")
		if len(kv) == 2 {
			state.Tag[kv[0]] = kv[1]
		}
	}
	return state, state.validate()
}

func getValueFromDriverOptions(driverOptions *types.DriverOptions, optionType string, keys ...string) interface{} {
	switch optionType {
	case types.IntType:
		for _, key := range keys {
			if value, ok := driverOptions.IntOptions[key]; ok {
				return value
			}
		}
		return int64(0)
	case types.StringType:
		for _, key := range keys {
			if value, ok := driverOptions.StringOptions[key]; ok {
				return value
			}
		}
		return ""
	case types.BoolType:
		for _, key := range keys {
			if value, ok := driverOptions.BoolOptions[key]; ok {
				return value
			}
		}
		return false
	case types.StringSliceType:
		for _, key := range keys {
			if value, ok := driverOptions.StringSliceOptions[key]; ok {
				return value
			}
		}
		return &types.StringSlice{}
	}
	return nil
}

func (state *state) validate() error {
	if state.Name == "" {
		return fmt.Errorf("cluster name is required")
	}

	if state.ResourceGroup == "" {
		return fmt.Errorf("resource group is required")
	}

	if state.SSHPublicKeyPath == "" {
		return fmt.Errorf("path to ssh public key is required")
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
	authorizer, err := utils.GetAuthorizer(azure.PublicCloud)

	if err != nil {
		return nil, err
	}

	client := containerservice.NewManagedClustersClientWithBaseURI(state.BaseURL, state.SubscriptionID)
	client.Authorizer = authorizer

	return &client, nil
}

const failedStatus = "Failed"
const succeededStatus = "Succeeded"
const creatingStatus = "Creating"

const pollInterval = 30

// Create implements driver interface
func (d *Driver) Create(ctx context.Context, options *types.DriverOptions) (*types.ClusterInfo, error) {
	driverState, err := getStateFromOptions(options)

	if err != nil {
		return nil, err
	}

	client, err := newAzureClient(driverState)

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

	publicKey, err := ioutil.ReadFile(driverState.SSHPublicKeyPath)

	if err != nil {
		return nil, err
	}

	publicKeyContents := string(publicKey)

	tags := make(map[string]*string)

	_, err = client.CreateOrUpdate(ctx, driverState.ResourceGroup, driverState.Name, containerservice.ManagedCluster{
		Location: to.StringPtr(driverState.Location),
		Tags:     &tags,
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
			AgentPoolProfiles: &[]containerservice.AgentPoolProfile{
				{
					DNSPrefix: to.StringPtr(agentDNSPrefix),
					Name:      to.StringPtr(driverState.AgentPoolName),
					VMSize:    containerservice.VMSizeTypes(driverState.AgentVMSize),
				},
			},
			ServicePrincipalProfile: &containerservice.ServicePrincipalProfile{
				ClientID: to.StringPtr(driverState.ClientID),
				Secret:   to.StringPtr(driverState.ClientSecret),
			},
		},
	})

	if err != nil {
		return nil, err
	}

	logrus.Info("Request submitted, waiting for cluster to finish creating")

	for {
		result, err := client.Get(ctx, driverState.ResourceGroup, driverState.Name)

		if err != nil {
			return nil, err
		}

		state := *result.ProvisioningState

		if state == failedStatus {
			return nil, fmt.Errorf("cluster create has completed with status of 'Failed'")
		}

		if state == succeededStatus {
			logrus.Info("Cluster provisioned successfully")
			info := &types.ClusterInfo{}
			err := storeState(info, driverState)

			fmt.Println("********")
			fmt.Println(info.Metadata["state"])
			fmt.Println("********")

			return info, err
		}

		if state != creatingStatus {
			return nil, fmt.Errorf("unexpected state %v", state)
		}

		logrus.Infof("Cluster has not yet completed provisioning, waiting another %v seconds", pollInterval)

		time.Sleep(pollInterval * time.Second)
	}
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

// Update implements driver interface
func (d *Driver) Update(ctx context.Context, info *types.ClusterInfo, options *types.DriverOptions) (*types.ClusterInfo, error) {
	// todo: implement
	return nil, fmt.Errorf("not implemented")
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

func (d *Driver) PostCheck(ctx context.Context, info *types.ClusterInfo) (*types.ClusterInfo, error) {
	state, err := getState(info)

	if err != nil {
		return nil, err
	}

	client, err := newAzureClient(state)

	if err != nil {
		return nil, err
	}

	result, err := client.GetAccessProfiles(context.Background(), state.ResourceGroup, state.Name, "clusterUser")

	if err != nil {
		return nil, err
	}

	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(*result.KubeConfig)))
	l, err := base64.StdEncoding.Decode(decoded, []byte(*result.KubeConfig))

	if err != nil {
		return nil, err
	}

	clusterConfig := KubeConfig{}
	err = yaml.Unmarshal(decoded[:l], &clusterConfig)

	if err != nil {
		return nil, err
	}

	singleCluster := clusterConfig.Clusters[0]
	singleUser := clusterConfig.Users[0]

	info.Version = clusterConfig.APIVersion
	info.Endpoint = singleCluster.ClusterInfo.Server
	info.Username = state.AdminUsername
	info.Password = singleUser.UserInfo.Token
	info.RootCaCertificate = singleCluster.ClusterInfo.CertificateAuthorityData
	info.ClientCertificate = singleUser.UserInfo.ClientCertificateData
	info.ClientKey = singleUser.UserInfo.ClientKeyData

	return info, nil
}

// Remove implements driver interface
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
