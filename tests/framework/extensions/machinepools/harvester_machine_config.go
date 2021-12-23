package machinepools

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

const (
	HarvesterKind           = "HarvesterConfig"
	HarvesterPoolType       = "rke-machine-config.cattle.io.harvesterconfig"
	HarvesterResourceConfig = "harvesterconfigs"
)

// NewHarvesterMachineConfig is a constructor to set up rke-machine-config.cattle.io.harvesterconfig.
// It returns an *unstructured.Unstructured that CreateMachineConfig uses to created the rke-machine-config
func NewHarvesterMachineConfig(generatedPoolName, namespace, network, image string) *unstructured.Unstructured {
	machineConfig := &unstructured.Unstructured{}
	machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
	machineConfig.SetKind(HarvesterKind)
	machineConfig.SetGenerateName(generatedPoolName)
	machineConfig.SetNamespace("fleet-default")

	machineConfig.Object["diskSize"] = "40"
	machineConfig.Object["cpuCount"] = "2"
	machineConfig.Object["memorySize"] = "8"
	machineConfig.Object["networkName"] = "default/ctw-network-1"
	machineConfig.Object["imageName"] = image
	machineConfig.Object["vmNamespace"] = namespace
	machineConfig.Object["sshUser"] = "ubuntu"
	machineConfig.Object["securityGroup"] = []string{
		"rancher-nodes",
	}

	return machineConfig
}
