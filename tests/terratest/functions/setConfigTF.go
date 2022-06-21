package functions

import (
	"os"

	"github.com/rancher/rancher/tests/terratest/components"
	functions "github.com/rancher/rancher/tests/terratest/functions/set_modules"
	"github.com/rancher/rancher/tests/terratest/models"
)

func SetConfigTF(module string, k8sVersion string, nodePools []models.Nodepool) (done bool, err error) {

	var result bool
	var file *os.File

	config := components.RequiredProviders + components.Provider

	// Hosted
	if module == "aks" || module == "eks" || module == "gke" {
		result, err = functions.SetHostedModules(module, k8sVersion, nodePools, result, file, config)
		return result, err
	}

	// Node_driver
	if module == "ec2_rke1" || module == "ec2_rke2" || module == "ec2_k3s" || module == "linode_rke1" || module == "linode_rke2" || module == "linode_k3s" {
		result, err = functions.SetNodeDriverModules(module, k8sVersion, nodePools, result, file, config)
		return result, err
	}

	// TODO: Add Custom

	return false, err
}
