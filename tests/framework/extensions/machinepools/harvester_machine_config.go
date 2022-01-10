package machinepools

import (
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	HarvesterKind                              = "HarvesterConfig"
	HarvesterPoolType                          = "rke-machine-config.cattle.io.harvesterconfig"
	HarvesterResourceConfig                    = "harvesterconfigs"
	HarvesterMachingConfigConfigurationFileKey = "harvesterMachineConfig"
)

type HarvesterMachineConfig struct {
	DiskSize    string `json:"diskSize" yaml:"diskSize"`
	CPUCount    string `json:"cpuCount" yaml:"cpuCount"`
	MemorySize  string `json:"memorySize" yaml:"memorySize"`
	NetworkName string `json:"networkName" yaml:"networkName"`
	ImageName   string `json:"imageName" yaml:"imageName"`
	VMNamespace string `json:"vmNamespace" yaml:"vmNamespace"`
	DiskBus     string `json:"diskBus" yaml:"diskBus"`
	SSHUser     string `json:"sshUser" yaml:"sshUser"`
}

// NewHarvesterMachineConfig is a constructor to set up rke-machine-config.cattle.io.harvesterconfig.
// It returns an *unstructured.Unstructured that CreateMachineConfig uses to created the rke-machine-config
func NewHarvesterMachineConfig(generatedPoolName, namespace string) *unstructured.Unstructured {
	var harvesterMachineConfig HarvesterMachineConfig
	config.LoadConfig(HarvesterMachingConfigConfigurationFileKey, &harvesterMachineConfig)
	machineConfig := &unstructured.Unstructured{}

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
	machineConfig.Object["vmNamespace"] = harvesterMachineConfig.VMNamespace
	machineConfig.Object["sshUser"] = harvesterMachineConfig.SSHUser

	return machineConfig
}
