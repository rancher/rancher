package machinepools

import (
	"github.com/rancher/shepherd/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	LinodeKind                              = "LinodeConfig"
	LinodePoolType                          = "rke-machine-config.cattle.io.linodeconfig"
	LinodeResourceConfig                    = "linodeconfigs"
	LinodeMachineConfigConfigurationFileKey = "linodeMachineConfigs"
)

type LinodeMachineConfigs struct {
	LinodeMachineConfig []LinodeMachineConfig `json:"linodeMachineConfig" yaml:"linodeMachineConfig"`
	Region              string                `json:"region" yaml:"region"`
}

// LinodeMachineConfig is configuration needed to create an rke-machine-config.cattle.io.linodeconfig
type LinodeMachineConfig struct {
	Roles
	AuthorizedUsers string `json:"authorizedUsers" yaml:"authorizedUsers"`
	DockerPort      string `json:"dockerPort" yaml:"dockerPort"`
	CreatePrivateIP bool   `json:"createPrivateIp" yaml:"createPrivateIp"`
	Image           string `json:"image" yaml:"image"`
	InstanceType    string `json:"instanceType" yaml:"instanceType"`
	RootPass        string `json:"rootPass" yaml:"rootPass"`
	SSHPort         string `json:"sshPort" yaml:"sshPort"`
	SSHUser         string `json:"sshUser" yaml:"sshUser"`
	Stackscript     string `json:"stackscript" yaml:"stackscript"`
	StackscriptData string `json:"stackscriptData" yaml:"stackscriptData"`
	SwapSize        string `json:"swapSize" yaml:"swapSize"`
	Tags            string `json:"tags" yaml:"tags"`
	UAPrefix        string `json:"uaPrefix" yaml:"uaPrefix"`
}

// NewLinodeMachineConfig is a constructor to set up rke-machine-config.cattle.io.linodeconfigs. It returns an *unstructured.Unstructured
// that CreateMachineConfig uses to created the rke-machine-config
func NewLinodeMachineConfig(generatedPoolName, namespace string) []unstructured.Unstructured {
	var linodeMachineConfigs LinodeMachineConfigs
	config.LoadConfig(LinodeMachineConfigConfigurationFileKey, &linodeMachineConfigs)
	var multiConfig []unstructured.Unstructured

	for _, linodeMachineConfig := range linodeMachineConfigs.LinodeMachineConfig {
		machineConfig := unstructured.Unstructured{}
		machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
		machineConfig.SetKind(LinodeKind)
		machineConfig.SetGenerateName(generatedPoolName)
		machineConfig.SetNamespace(namespace)

		machineConfig.Object["authorizedUsers"] = linodeMachineConfig.AuthorizedUsers
		machineConfig.Object["createPrivateIp"] = linodeMachineConfig.CreatePrivateIP
		machineConfig.Object["dockerPort"] = linodeMachineConfig.DockerPort
		machineConfig.Object["image"] = linodeMachineConfig.Image
		machineConfig.Object["instanceType"] = linodeMachineConfig.InstanceType
		machineConfig.Object["region"] = linodeMachineConfigs.Region
		machineConfig.Object["rootPass"] = linodeMachineConfig.RootPass
		machineConfig.Object["sshPort"] = linodeMachineConfig.SSHPort
		machineConfig.Object["sshUser"] = linodeMachineConfig.SSHUser
		machineConfig.Object["stackscript"] = linodeMachineConfig.Stackscript
		machineConfig.Object["stackscriptData"] = linodeMachineConfig.StackscriptData
		machineConfig.Object["swapSize"] = linodeMachineConfig.SwapSize
		machineConfig.Object["tags"] = linodeMachineConfig.Tags
		machineConfig.Object["token"] = ""
		machineConfig.Object["type"] = LinodePoolType
		machineConfig.Object["uaPrefix"] = linodeMachineConfig.UAPrefix

		multiConfig = append(multiConfig, machineConfig)
	}

	return multiConfig
}

// GetLinodeMachineRoles returns a list of roles from the given machineConfigs
func GetLinodeMachineRoles() []Roles {
	var linodeMachineConfigs LinodeMachineConfigs
	config.LoadConfig(LinodeMachineConfigConfigurationFileKey, &linodeMachineConfigs)
	var allRoles []Roles

	for _, linodeMachineConfig := range linodeMachineConfigs.LinodeMachineConfig {
		allRoles = append(allRoles, linodeMachineConfig.Roles)
	}

	return allRoles
}
