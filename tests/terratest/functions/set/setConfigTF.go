package functions

import (
	"fmt"
	"os"
	"testing"

	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/v2/validation/terratest"
)

func SetConfigTF(t *testing.T, k8sVersion string, nodePools []terratest.Nodepool) (done bool, err error) {
	terraformConfig := new(terratest.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

	module := terraformConfig.Module

	var result bool
	var file *os.File

	keyPath := SetKeyPath()

	file, err = os.Create(keyPath + "/main.tf")

	if err != nil {
		t.Logf("Failed to reset/overwrite main.tf file. Error: %v", err)
		return false, err
	}

	defer file.Close()

	switch {
	case module == "aks":
		result, err = SetAKS(t, k8sVersion, nodePools, file)
		return result, err

	case module == "eks":
		result, err = SetEKS(t, k8sVersion, nodePools, file)
		return result, err

	case module == "gke":
		result, err = SetGKE(t, k8sVersion, nodePools, file)
		return result, err

	case module == "ec2_rke1" || module == "linode_rke1":
		result, err = SetRKE1(t, k8sVersion, nodePools, file)
		return result, err

	case module == "ec2_rke2" || module == "ec2_k3s" || module == "linode_rke2" || module == "linode_k3s":
		result, err = SetRKE2K3s(t, k8sVersion, nodePools, file)
		return result, err

	default:
		t.Logf("Invalid module provided. Valid modules are: aks, eks, gke, ec2_rke1, linode_rke1, ec2_rke2, linode_rke2, ec2_k3s, linode_k3s")
		return false, fmt.Errorf("invalid module provided")
	}
}
