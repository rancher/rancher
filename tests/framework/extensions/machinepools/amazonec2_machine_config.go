package machinepools

import (
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	AWSKind                              = "Amazonec2Config"
	AWSPoolType                          = "rke-machine-config.cattle.io.amazonec2config"
	AWSResourceConfig                    = "amazonec2configs"
	AWSMachineConfigConfigurationFileKey = "awsMachineConfig"
)

// AWSMachineConfig is configuration needed to create an rke-machine-config.cattle.io.amazonec2config
type AWSMachineConfig struct {
	Region             string   `json:"region" yaml:"region"`
	AMI                string   `json:"ami" yaml:"ami"`
	IAMInstanceProfile string   `json:"iamInstanceProfile" yaml:"iamInstanceProfile"`
	InstanceType       string   `json:"instanceType" yaml:"instanceType"`
	SSHUser            string   `json:"sshUser" yaml:"sshUser"`
	VPCID              string   `json:"vpcId" yaml:"vpcId"`
	SubnetID           string   `json:"subnetId" yaml:"subnetId"`
	VolumeType         string   `json:"volumeType" yaml:"volumeType"`
	Zone               string   `json:"zone" yaml:"zone"`
	Retries            string   `json:"retries" yaml:"retries"`
	RootSize           string   `json:"rootSize" yaml:"rootSize"`
	SecurityGroup      []string `json:"securityGroup" yaml:"securityGroup"`
}

// NewAWSMachineConfig is a constructor to set up rke-machine-config.cattle.io.amazonec2config. It returns an *unstructured.Unstructured
// that CreateMachineConfig uses to created the rke-machine-config
func NewAWSMachineConfig(generatedPoolName, namespace string) *unstructured.Unstructured {
	var awsMachineConfig AWSMachineConfig
	config.LoadConfig(AWSMachineConfigConfigurationFileKey, &awsMachineConfig)

	machineConfig := &unstructured.Unstructured{}
	machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
	machineConfig.SetKind(AWSKind)
	machineConfig.SetGenerateName(generatedPoolName)
	machineConfig.SetNamespace(namespace)
	machineConfig.Object["region"] = awsMachineConfig.Region
	machineConfig.Object["ami"] = awsMachineConfig.AMI
	machineConfig.Object["iamInstanceProfile"] = awsMachineConfig.IAMInstanceProfile
	machineConfig.Object["instanceType"] = awsMachineConfig.InstanceType
	machineConfig.Object["sshUser"] = awsMachineConfig.SSHUser
	machineConfig.Object["type"] = AWSPoolType
	machineConfig.Object["vpcId"] = awsMachineConfig.VPCID
	machineConfig.Object["subnetId"] = awsMachineConfig.SubnetID
	machineConfig.Object["volumeType"] = awsMachineConfig.VolumeType
	machineConfig.Object["zone"] = awsMachineConfig.Zone
	machineConfig.Object["retries"] = awsMachineConfig.Retries
	machineConfig.Object["rootSize"] = awsMachineConfig.RootSize
	machineConfig.Object["securityGroup"] = awsMachineConfig.SecurityGroup

	return machineConfig
}
