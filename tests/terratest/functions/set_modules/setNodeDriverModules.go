package functions

import (
	"fmt"
	"os"

	"github.com/rancher/rancher/tests/terratest/models"
)

func SetNodeDriverModules(module string, k8sVersion string, nodePools []models.Nodepool, result bool, file *os.File, config string) (done bool, err error) {
	file, err = os.Create("../../modules/node_driver/" + module + "/main.tf")

	if err != nil {
		return false, err
	}

	defer file.Close()

	switch {
	case module == "ec2_rke1":
		result, err = SetEC2RKE1(module, k8sVersion, nodePools, config, file)
		return result, err

	case module == "ec2_rke2" || module == "ec2_k3s":
		result, err = SetEC2RKE2K3s(module, k8sVersion, nodePools, config, file)
		return result, err

	case module == "linode_rke1":
		result, err = SetLinodeRKE1(module, k8sVersion, nodePools, config, file)
		return result, err

	case module == "linode_rke2" || module == "linode_k3s":
		result, err = SetLinodeRKE2K3s(module, k8sVersion, nodePools, config, file)
		return result, err

	default:
		return false, fmt.Errorf("invalid node_driver module provided")
	}
}
