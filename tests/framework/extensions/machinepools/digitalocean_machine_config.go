package machinepools

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

const (
	DOKind           = "DigitaloceanConfig"
	DOPoolType       = "rke-machine-config.cattle.io.digitaloceanconfig"
	DOResourceConfig = "digitaloceanconfigs"
)

// NewDigitalOceanMachineConfig is a constructor to set up rke-machine-config.cattle.io.digitaloceanconfig. It returns an *unstructured.Unstructured
// that CreateMachineConfig uses to created the rke-machine-config
func NewDigitalOceanMachineConfig(generatedPoolName, namespace, image, region, size string) *unstructured.Unstructured {
	machineConfig := &unstructured.Unstructured{}
	machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
	machineConfig.SetKind(DOKind)
	machineConfig.SetGenerateName(generatedPoolName)
	machineConfig.SetNamespace(namespace)
	machineConfig.Object["accessToken"] = ""
	machineConfig.Object["image"] = image
	machineConfig.Object["backups"] = false
	machineConfig.Object["ipv6"] = false
	machineConfig.Object["monitoring"] = false
	machineConfig.Object["privateNetworking"] = false
	machineConfig.Object["region"] = region
	machineConfig.Object["size"] = size
	machineConfig.Object["sshKeyContents"] = ""
	machineConfig.Object["sshKeyFingerprint"] = ""
	machineConfig.Object["sshPort"] = "22"
	machineConfig.Object["sshUser"] = "root"
	machineConfig.Object["tags"] = ""
	machineConfig.Object["type"] = DOPoolType
	machineConfig.Object["userdata"] = ""
	return machineConfig
}
