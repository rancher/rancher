package machinepools

import (
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	AWSKind                              = "Amazonec2Config"
	AWSPoolType                          = "rke-machine-config.cattle.io.amazonec2config"
	AWSResourceConfig                    = "amazonec2configs"
	AWSMachingConfigConfigurationFileKey = "awsMachineConfig"
)

type AWSMachineConfig struct {
	Region        string   `json:"region" yaml:"region"`
	InstanceType  string   `json:"instanceType" yaml:"instanceType"`
	SSHUser       string   `json:"sshUser" yaml:"sshUser"`
	VPCID         string   `json:"vpcId" yaml:"vpcId"`
	VolumeType    string   `json:"volumeType" yaml:"volumeType"`
	Zone          string   `json:"zone" yaml:"zone"`
	Retries       string   `json:"retries" yaml:"retries"`
	RootSize      string   `json:"rootSize" yaml:"rootSize"`
	SecurityGroup []string `json:"securityGroup" yaml:"securityGroup"`
}

// NewAWSMachineConfig is a constructor to set up rke-machine-config.cattle.io.amazonec2config. It returns an *unstructured.Unstructured
// that CreateMachineConfig uses to created the rke-machine-config
func NewAWSMachineConfig(generatedPoolName, namespace string) *unstructured.Unstructured {
	var awsMachineConfig AWSMachineConfig
	config.LoadConfig(AWSMachingConfigConfigurationFileKey, &awsMachineConfig)

	machineConfig := &unstructured.Unstructured{}
	machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
	machineConfig.SetKind(AWSKind)
	machineConfig.SetGenerateName(generatedPoolName)
	machineConfig.SetNamespace(namespace)
	machineConfig.Object["region"] = awsMachineConfig.Region
	machineConfig.Object["instanceType"] = awsMachineConfig.InstanceType
	machineConfig.Object["sshUser"] = awsMachineConfig.SSHUser
	machineConfig.Object["type"] = AWSPoolType
	machineConfig.Object["vpcId"] = awsMachineConfig.VPCID
	machineConfig.Object["volumeType"] = awsMachineConfig.VolumeType
	machineConfig.Object["zone"] = awsMachineConfig.Zone
	machineConfig.Object["retries"] = awsMachineConfig.Retries
	machineConfig.Object["rootSize"] = awsMachineConfig.RootSize
	machineConfig.Object["securityGroup"] = awsMachineConfig.SecurityGroup

	return machineConfig
}
