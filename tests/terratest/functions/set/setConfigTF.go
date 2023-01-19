package functions

import (
	"fmt"
	"os"

	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/v2/validation/terratest"
)

func SetConfigTF(k8sVersion string, nodePools []terratest.Nodepool) (done bool, err error) {
	terraformConfig := new(terratest.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

	module := terraformConfig.Module

	var result bool
	var file *os.File

	file, err = os.Create("../../../../terratest/modules/cluster/main.tf")

	if err != nil {
		return false, err
	}

	defer file.Close()

	switch {
	case module == "aks":
		result, err = SetAKS(k8sVersion, nodePools, file)
		return result, err

	case module == "eks":
		result, err = SetEKS(k8sVersion, nodePools, file)
		return result, err

	case module == "gke":
		result, err = SetGKE(k8sVersion, nodePools, file)
		return result, err

	case module == "ec2_rke1" || module == "linode_rke1":
		result, err = SetRKE1(k8sVersion, nodePools, file)
		return result, err

	case module == "ec2_rke2" || module == "ec2_k3s" || module == "linode_rke2" || module == "linode_k3s":
		result, err = SetRKE2K3s(k8sVersion, nodePools, file)
		return result, err

	default:
		return false, fmt.Errorf("invalid module provided")
	}
}
