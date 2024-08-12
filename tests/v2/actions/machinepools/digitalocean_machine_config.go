package machinepools

import (
	"github.com/rancher/shepherd/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	DOKind                              = "DigitaloceanConfig"
	DOPoolType                          = "rke-machine-config.cattle.io.digitaloceanconfig"
	DOResourceConfig                    = "digitaloceanconfigs"
	DOMachineConfigConfigurationFileKey = "doMachineConfigs"
)

type DOMachineConfigs struct {
	DOMachineConfig []DOMachineConfig `json:"doMachineConfig" yaml:"doMachineConfig"`
	Region          string            `json:"region" yaml:"region"`
}

// DOMachineConfig is configuration needed to create an rke-machine-config.cattle.io.digitaloceanconfig
type DOMachineConfig struct {
	Roles
	Image             string `json:"image" yaml:"image"`
	Backups           bool   `json:"backups" yaml:"backups"`
	IPV6              bool   `json:"ipv6" yaml:"ipv6"`
	Monitoring        bool   `json:"monitoring" yaml:"monitoring"`
	PrivateNetworking bool   `json:"privateNetworking" yaml:"privateNetworking"`
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
func NewDigitalOceanMachineConfig(generatedPoolName, namespace string) []unstructured.Unstructured {
	var doMachineConfigs DOMachineConfigs
	config.LoadConfig(DOMachineConfigConfigurationFileKey, &doMachineConfigs)
	var multiConfig []unstructured.Unstructured

	for _, doMachineConfig := range doMachineConfigs.DOMachineConfig {
		machineConfig := unstructured.Unstructured{}
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
		machineConfig.Object["region"] = doMachineConfigs.Region
		machineConfig.Object["size"] = doMachineConfig.Size
		machineConfig.Object["sshKeyContents"] = doMachineConfig.SSHKeyContents
		machineConfig.Object["sshKeyFingerprint"] = doMachineConfig.SSHKeyFingerprint
		machineConfig.Object["sshPort"] = doMachineConfig.SSHPort
		machineConfig.Object["sshUser"] = doMachineConfig.SSHUser
		machineConfig.Object["tags"] = doMachineConfig.Tags
		machineConfig.Object["type"] = DOPoolType
		machineConfig.Object["userdata"] = doMachineConfig.Userdata

		multiConfig = append(multiConfig, machineConfig)
	}

	return multiConfig
}

// GetDOMachineRoles returns a list of roles from the given machineConfigs
func GetDOMachineRoles() []Roles {
	var doMachineConfigs DOMachineConfigs
	config.LoadConfig(DOMachineConfigConfigurationFileKey, &doMachineConfigs)
	var allRoles []Roles

	for _, doMachineConfig := range doMachineConfigs.DOMachineConfig {
		allRoles = append(allRoles, doMachineConfig.Roles)
	}

	return allRoles
}
