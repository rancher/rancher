package machinepools

import (
	"github.com/rancher/shepherd/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	AzureKind                              = "AzureConfig"
	AzurePoolType                          = "rke-machine-config.cattle.io.azureconfig"
	AzureResourceConfig                    = "azureconfigs"
	AzureMachineConfigConfigurationFileKey = "azureMachineConfigs"
)

type AzureMachineConfigs struct {
	AzureMachineConfig []AzureMachineConfig `json:"azureMachineConfig" yaml:"azureMachineConfig"`
	Environment        string               `json:"environment" yaml:"environment"`
}

// AzureMachineConfig is configuration needed to create an rke-machine-config.cattle.io.azureconfig
type AzureMachineConfig struct {
	Roles
	AvailabilitySet   string   `json:"availabilitySet" yaml:"availabilitySet"`
	DiskSize          string   `json:"diskSize" yaml:"diskSize"`
	DNS               string   `json:"dns,omitempty" yaml:"dns,omitempty"`
	FaultDomainCount  string   `json:"faultDomainCount" yaml:"faultDomainCount"`
	Location          string   `json:"location" yaml:"location"`
	Image             string   `json:"image" yaml:"image"`
	ManagedDisks      bool     `json:"managedDisks" yaml:"managedDisks"`
	NoPublicIP        bool     `json:"noPublicIp" yaml:"noPublicIp"`
	NSG               string   `json:"nsg" yaml:"nsg"`
	OpenPort          []string `json:"openPort" yaml:"openPort"`
	PrivateIPAddress  string   `json:"privateIpAddress,omitempty" yaml:"privateIpAddress,omitempty"`
	ResourceGroup     string   `json:"resourceGroup" yaml:"resourceGroup"`
	Size              string   `json:"size" yaml:"size"`
	SSHUser           string   `json:"sshUser" yaml:"sshUser"`
	StaticPublicIP    bool     `json:"staticPublicIp" yaml:"staticPublicIp"`
	StorageType       string   `json:"storageType" yaml:"storageType"`
	Subnet            string   `json:"subnet" yaml:"subnet"`
	SubnetPrefix      string   `json:"subnetPrefix" yaml:"subnetPrefix"`
	UpdateDomainCount string   `json:"updateDomainCount" yaml:"updateDomainCount"`
	UsePrivateIP      bool     `json:"usePrivateIp" yaml:"usePrivateIp"`
	Vnet              string   `json:"vnet" yaml:"vnet"`
}

// NewAzureMachineConfig is a constructor to set up rke-machine-config.cattle.io.azureconfig. It returns an *unstructured.Unstructured
// that CreateMachineConfig uses to created the rke-machine-config
func NewAzureMachineConfig(generatedPoolName, namespace string) []unstructured.Unstructured {
	var azureMachineConfigs AzureMachineConfigs
	config.LoadConfig(AzureMachineConfigConfigurationFileKey, &azureMachineConfigs)
	var multiConfig []unstructured.Unstructured

	for _, azureMachineConfig := range azureMachineConfigs.AzureMachineConfig {
		machineConfig := unstructured.Unstructured{}
		machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
		machineConfig.SetKind(AzureKind)
		machineConfig.SetGenerateName(generatedPoolName)
		machineConfig.SetNamespace(namespace)

		machineConfig.Object["availabilitySet"] = azureMachineConfig.AvailabilitySet
		machineConfig.Object["diskSize"] = azureMachineConfig.DiskSize
		machineConfig.Object["dns"] = azureMachineConfig.DNS
		machineConfig.Object["location"] = azureMachineConfig.Location
		machineConfig.Object["environment"] = azureMachineConfigs.Environment
		machineConfig.Object["faultDomainCount"] = azureMachineConfig.FaultDomainCount
		machineConfig.Object["image"] = azureMachineConfig.Image
		machineConfig.Object["managedDisks"] = azureMachineConfig.ManagedDisks
		machineConfig.Object["noPublicIp"] = azureMachineConfig.NoPublicIP
		machineConfig.Object["nsg"] = azureMachineConfig.NSG
		machineConfig.Object["openPort"] = azureMachineConfig.OpenPort
		machineConfig.Object["privateIpAddress"] = azureMachineConfig.PrivateIPAddress
		machineConfig.Object["resourceGroup"] = azureMachineConfig.ResourceGroup
		machineConfig.Object["size"] = azureMachineConfig.Size
		machineConfig.Object["sshUser"] = azureMachineConfig.SSHUser
		machineConfig.Object["staticPublicIP"] = azureMachineConfig.StaticPublicIP
		machineConfig.Object["storageType"] = azureMachineConfig.StorageType
		machineConfig.Object["subnet"] = azureMachineConfig.Subnet
		machineConfig.Object["subnetPrefix"] = azureMachineConfig.SubnetPrefix
		machineConfig.Object["updateDomainCount"] = azureMachineConfig.UpdateDomainCount
		machineConfig.Object["usePrivateIp"] = azureMachineConfig.UsePrivateIP
		machineConfig.Object["vnet"] = azureMachineConfig.Vnet
		machineConfig.Object["type"] = AzurePoolType

		multiConfig = append(multiConfig, machineConfig)
	}

	return multiConfig
}

// GetAzureMachineRoles returns a list of roles from the given machineConfigs
func GetAzureMachineRoles() []Roles {
	var azureMachineConfigs AzureMachineConfigs
	config.LoadConfig(AzureMachineConfigConfigurationFileKey, &azureMachineConfigs)
	var allRoles []Roles

	for _, azureMachineConfig := range azureMachineConfigs.AzureMachineConfig {
		allRoles = append(allRoles, azureMachineConfig.Roles)
	}

	return allRoles
}
