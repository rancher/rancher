package machinepools

import (
	"github.com/rancher/shepherd/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	AWSKind                              = "Amazonec2Config"
	AWSPoolType                          = "rke-machine-config.cattle.io.amazonec2config"
	AWSResourceConfig                    = "amazonec2configs"
	AWSMachineConfigConfigurationFileKey = "awsMachineConfigs"
)

type AWSMachineConfigs struct {
	AWSMachineConfig []AWSMachineConfig `json:"awsMachineConfig" yaml:"awsMachineConfig"`
	Region           string             `json:"region" yaml:"region"`
}

// AWSMachineConfig is configuration needed to create an rke-machine-config.cattle.io.amazonec2config
type AWSMachineConfig struct {
	Roles
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
func NewAWSMachineConfig(generatedPoolName, namespace string) []unstructured.Unstructured {
	var awsMachineConfigs AWSMachineConfigs
	config.LoadConfig(AWSMachineConfigConfigurationFileKey, &awsMachineConfigs)
	var multiConfig []unstructured.Unstructured

	for _, awsMachineConfig := range awsMachineConfigs.AWSMachineConfig {
		machineConfig := unstructured.Unstructured{}
		machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
		machineConfig.SetKind(AWSKind)
		machineConfig.SetGenerateName(generatedPoolName)
		machineConfig.SetNamespace(namespace)

		machineConfig.Object["region"] = awsMachineConfigs.Region
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

		multiConfig = append(multiConfig, machineConfig)
	}

	return multiConfig
}

// GetAWSMachineRoles returns a list of roles from the given machineConfigs
func GetAWSMachineRoles() []Roles {
	var awsMachineConfigs AWSMachineConfigs
	config.LoadConfig(AWSMachineConfigConfigurationFileKey, &awsMachineConfigs)
	var allRoles []Roles

	for _, awsMachineConfig := range awsMachineConfigs.AWSMachineConfig {
		allRoles = append(allRoles, awsMachineConfig.Roles)
	}

	return allRoles
}
