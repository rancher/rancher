package machinepools

import (
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	LinodeKind                              = "LinodeConfig"
	LinodePoolType                          = "rke-machine-config.cattle.io.linodeconfig"
	LinodeResourceConfig                    = "linodeconfigs"
	LinodeMachineConfigConfigurationFileKey = "linodeMachineConfig"
)

// LinodeMachineConfig is configuration needed to create an rke-machine-config.cattle.io.linodeconfig
type LinodeMachineConfig struct {
	AuthorizedUsers string `json:"authorizedUsers" yaml:"authorizedUsers"`
	DockerPort      string `json:"dockerPort" yaml:"dockerPort"`
	CreatePrivateIP bool   `json:"createPrivateIp" yaml:"createPrivateIp"`
	Image           string `json:"image" yaml:"image"`
	InstanceType    string `json:"instanceType" yaml:"instanceType"`
	Region          string `json:"region" yaml:"region"`
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
func NewLinodeMachineConfig(generatedPoolName, namespace string) *unstructured.Unstructured {
	var linodeMachineConfig LinodeMachineConfig
	config.LoadConfig(LinodeMachineConfigConfigurationFileKey, &linodeMachineConfig)

	machineConfig := &unstructured.Unstructured{}
	machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
	machineConfig.SetKind(LinodeKind)
	machineConfig.SetGenerateName(generatedPoolName)
	machineConfig.SetNamespace(namespace)
	machineConfig.Object["authorizedUsers"] = linodeMachineConfig.AuthorizedUsers
	machineConfig.Object["createPrivateIp"] = linodeMachineConfig.CreatePrivateIP
	machineConfig.Object["dockerPort"] = linodeMachineConfig.DockerPort
	machineConfig.Object["image"] = linodeMachineConfig.Image
	machineConfig.Object["instanceType"] = linodeMachineConfig.InstanceType
	machineConfig.Object["region"] = linodeMachineConfig.Region
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
	return machineConfig
}
