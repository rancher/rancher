package machinepools

import (
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	AzureKind                              = "AzureConfig"
	AzurePoolType                          = "rke-machine-config.cattle.io.azureconfig"
	AzureResourceConfig                    = "azureconfigs"
	AzureMachineConfigConfigurationFileKey = "azureMachineConfig"
)

// AzureMachineConfig is configuration needed to create an rke-machine-config.cattle.io.azureconfig
type AzureMachineConfig struct {
	AvailabilitySet   string   `json:"availabilitySet" yaml:"availabilitySet"`
	DiskSize          string   `json:"diskSize" yaml:"diskSize"`
	DNS               string   `json:"dns,omitempty" yaml:"dns,omitempty"`
	Environment       string   `json:"environment" yaml:"environment"`
	FaultDomainCount  string   `json:"faultDomainCount" yaml:"faultDomainCount"`
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
func NewAzureMachineConfig(generatedPoolName, namespace string) *unstructured.Unstructured {
	var azureMachineConfig AzureMachineConfig
	config.LoadConfig(AzureMachineConfigConfigurationFileKey, &azureMachineConfig)

	machineConfig := &unstructured.Unstructured{}
	machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
	machineConfig.SetKind(DOKind)
	machineConfig.SetGenerateName(generatedPoolName)
	machineConfig.SetNamespace(namespace)
	machineConfig.Object["availabilitySet"] = azureMachineConfig.AvailabilitySet
	machineConfig.Object["diskSize"] = azureMachineConfig.DiskSize
	machineConfig.Object["dns"] = azureMachineConfig.DNS
	machineConfig.Object["environment"] = azureMachineConfig.Environment
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
	return machineConfig
}
