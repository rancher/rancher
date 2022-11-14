package functions

import (
	"fmt"
	"os"

	"github.com/rancher/rancher/tests/terratest/models"
)

func SetHostedModules(module string, k8sVersion string, nodePools []models.Nodepool, result bool, file *os.File, config string) (done bool, err error) {
	file, err = os.Create("../../modules/hosted/" + module + "/main.tf")

	if err != nil {
		return false, err
	}

	defer file.Close()

	switch {
	case module == "aks":
		result, err = SetAKS(module, k8sVersion, nodePools, config, file)
		return result, err

	case module == "eks":
		result, err = SetEKS(module, k8sVersion, nodePools, config, file)
		return result, err

	case module == "gke":
		result, err = SetGKE(module, k8sVersion, nodePools, config, file)
		return result, err

	default:
		return false, fmt.Errorf("invalid hosted module provided")
	}
}
