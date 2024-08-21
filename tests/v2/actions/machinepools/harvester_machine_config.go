package machinepools

import (
	"github.com/rancher/shepherd/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	HarvesterKind                              = "HarvesterConfig"
	HarvesterPoolType                          = "rke-machine-config.cattle.io.harvesterconfig"
	HarvesterResourceConfig                    = "harvesterconfigs"
	HarvesterMachineConfigConfigurationFileKey = "harvesterMachineConfigs"
)

type HarvesterMachineConfigs struct {
	HarvesterMachineConfig []HarvesterMachineConfig `json:"harvesterMachineConfig" yaml:"harvesterMachineConfig"`
	VMNamespace            string                   `json:"vmNamespace" yaml:"vmNamespace"`
}

// HarvesterMachineConfig is configuration needed to create an rke-machine-config.cattle.io.harvesterconfig
type HarvesterMachineConfig struct {
	Roles
	DiskSize    string `json:"diskSize" yaml:"diskSize"`
	CPUCount    string `json:"cpuCount" yaml:"cpuCount"`
	MemorySize  string `json:"memorySize" yaml:"memorySize"`
	NetworkName string `json:"networkName" yaml:"networkName"`
	ImageName   string `json:"imageName" yaml:"imageName"`
	DiskBus     string `json:"diskBus" yaml:"diskBus"`
	SSHUser     string `json:"sshUser" yaml:"sshUser"`
}

// NewHarvesterMachineConfig is a constructor to set up rke-machine-config.cattle.io.harvesterconfig.
// It returns an *unstructured.Unstructured that CreateMachineConfig uses to created the rke-machine-config
func NewHarvesterMachineConfig(generatedPoolName, namespace string) []unstructured.Unstructured {
	var harvesterMachineConfigs HarvesterMachineConfigs
	config.LoadConfig(HarvesterMachineConfigConfigurationFileKey, &harvesterMachineConfigs)
	var multiConfig []unstructured.Unstructured

	for _, harvesterMachineConfig := range harvesterMachineConfigs.HarvesterMachineConfig {
		machineConfig := unstructured.Unstructured{}
		machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
		machineConfig.SetKind(HarvesterKind)
		machineConfig.SetGenerateName(generatedPoolName)
		machineConfig.SetNamespace(namespace)

		machineConfig.Object["diskSize"] = harvesterMachineConfig.DiskSize
		machineConfig.Object["diskBus"] = harvesterMachineConfig.DiskBus
		machineConfig.Object["cpuCount"] = harvesterMachineConfig.CPUCount
		machineConfig.Object["memorySize"] = harvesterMachineConfig.MemorySize
		machineConfig.Object["networkName"] = harvesterMachineConfig.NetworkName
		machineConfig.Object["imageName"] = harvesterMachineConfig.ImageName
		machineConfig.Object["vmNamespace"] = harvesterMachineConfigs.VMNamespace
		machineConfig.Object["sshUser"] = harvesterMachineConfig.SSHUser

		multiConfig = append(multiConfig, machineConfig)
	}

	return multiConfig
}

// GetHarvesterMachineRoles returns a list of roles from the given machineConfigs
func GetHarvesterMachineRoles() []Roles {
	var harvesterMachineConfigs HarvesterMachineConfigs
	config.LoadConfig(HarvesterMachineConfigConfigurationFileKey, &harvesterMachineConfigs)
	var allRoles []Roles

	for _, harvesterMachineConfig := range harvesterMachineConfigs.HarvesterMachineConfig {
		allRoles = append(allRoles, harvesterMachineConfig.Roles)
	}

	return allRoles
}
