package machinepools

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	HarvesterKind           = "HarvesterConfig"
	HarvesterPoolType       = "rke-machine-config.cattle.io.harvesterconfig"
	HarvesterResourceConfig = "harvesterconfigs"
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
func NewHarvesterMachineConfig(machineConfigs MachineConfigs, generatedPoolName, namespace string) []unstructured.Unstructured {
	var multiConfig []unstructured.Unstructured
	for _, harvesterMachineConfig := range machineConfigs.HarvesterMachineConfigs.HarvesterMachineConfig {
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
		machineConfig.Object["vmNamespace"] = machineConfigs.HarvesterMachineConfigs.VMNamespace
		machineConfig.Object["sshUser"] = harvesterMachineConfig.SSHUser

		multiConfig = append(multiConfig, machineConfig)
	}

	return multiConfig
}

// GetHarvesterMachineRoles returns a list of roles from the given machineConfigs
func GetHarvesterMachineRoles(machineConfigs MachineConfigs) []Roles {
	var allRoles []Roles

	for _, harvesterMachineConfig := range machineConfigs.HarvesterMachineConfigs.HarvesterMachineConfig {
		allRoles = append(allRoles, harvesterMachineConfig.Roles)
	}

	return allRoles
}
