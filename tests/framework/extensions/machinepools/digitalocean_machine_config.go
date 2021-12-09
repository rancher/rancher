package machinepools

import (
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	DOKind                              = "DigitaloceanConfig"
	DOPoolType                          = "rke-machine-config.cattle.io.digitaloceanconfig"
	DOResourceConfig                    = "digitaloceanconfigs"
	DOMachingConfigConfigurationFileKey = "doMachineConfig"
)

type DOMachineConfig struct {
	Image             string `json:"image" yaml:"image"`
	Backups           bool   `json:"backups" yaml:"backups"`
	IPV6              bool   `json:"ipv6" yaml:"ipv6"`
	Monitoring        bool   `json:"monitoring" yaml:"monitoring"`
	PrivateNetworking bool   `json:"privateNetworking" yaml:"privateNetworking"`
	Region            string `json:"region" yaml:"region"`
	Size              string `json:"size" yaml:"size"`
	SSHKeyContents    string `json:"sshKeyContents" yaml:"sshKeyContents"`
	SSHKeyFingerprint string `json:"sshKeyFingerprint" yaml:"sshKeyFingerprint"`
	SSHPort           string `json:"sshPort" yaml:"sshPort"`
	SSHUser           string `json:"sshUser" yaml:"sshUser"`
	Tags              string `json:"tags" yaml:"tags"`
	Userdata          string `json:"userdata" yaml:"userdata"`
}

// NewDigitalOceanMachineConfig is a constructor to set up rke-machine-config.cattle.io.digitaloceanconfig. It returns an *unstructured.Unstructured
// that CreateMachineConfig uses to created the rke-machine-config
func NewDigitalOceanMachineConfig(generatedPoolName, namespace string) *unstructured.Unstructured {
	var doMachineConfig DOMachineConfig
	config.LoadConfig(DOMachingConfigConfigurationFileKey, &doMachineConfig)

	machineConfig := &unstructured.Unstructured{}
	machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
	machineConfig.SetKind(DOKind)
	machineConfig.SetGenerateName(generatedPoolName)
	machineConfig.SetNamespace(namespace)
	machineConfig.Object["accessToken"] = ""
	machineConfig.Object["image"] = doMachineConfig.Image
	machineConfig.Object["backups"] = doMachineConfig.Backups
	machineConfig.Object["ipv6"] = doMachineConfig.IPV6
	machineConfig.Object["monitoring"] = doMachineConfig.Monitoring
	machineConfig.Object["privateNetworking"] = doMachineConfig.PrivateNetworking
	machineConfig.Object["region"] = doMachineConfig.Region
	machineConfig.Object["size"] = doMachineConfig.Size
	machineConfig.Object["sshKeyContents"] = doMachineConfig.SSHKeyContents
	machineConfig.Object["sshKeyFingerprint"] = doMachineConfig.SSHKeyFingerprint
	machineConfig.Object["sshPort"] = doMachineConfig.SSHPort
	machineConfig.Object["sshUser"] = doMachineConfig.SSHUser
	machineConfig.Object["tags"] = doMachineConfig.Tags
	machineConfig.Object["type"] = DOPoolType
	machineConfig.Object["userdata"] = doMachineConfig.Userdata
	return machineConfig
}
