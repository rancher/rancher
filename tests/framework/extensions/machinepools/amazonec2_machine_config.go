package machinepools

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

const (
	AWSKind           = "Amazonec2Config"
	AWSPoolType       = "rke-machine-config.cattle.io.amazonec2config"
	AWSResourceConfig = "amazonec2configs"
)

func NewAWSMachineConfig(generatedPoolName, namespace, region string) *unstructured.Unstructured {
	machineConfig := &unstructured.Unstructured{}
	machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
	machineConfig.SetKind(AWSKind)
	machineConfig.SetGenerateName(generatedPoolName)
	machineConfig.SetNamespace(namespace)
	machineConfig.Object["region"] = region
	machineConfig.Object["instanceType"] = "t3a.medium"
	machineConfig.Object["sshUser"] = "ubuntu"
	machineConfig.Object["type"] = AWSPoolType
	machineConfig.Object["vpcId"] = "vpc-bfccf4d7"
	machineConfig.Object["volumeType"] = "gp2"
	machineConfig.Object["zone"] = "a"
	machineConfig.Object["retries"] = "5"
	machineConfig.Object["rootSize"] = "16"
	machineConfig.Object["securityGroup"] = []string{
		"rancher-nodes",
	}

	return machineConfig
}
