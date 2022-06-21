package functions

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rancher/rancher/tests/terratest/components"
	"github.com/rancher/rancher/tests/terratest/models"
)

func SetAKS(module string, k8sVersion string, nodePools []models.Nodepool, config string, file *os.File) (done bool, err error) {
	config = config + components.AzureCloudCredentials
	poolConfig := ``
	num := 1
	for _, pool := range nodePools {
		poolNum := strconv.Itoa(num)
		quantity := strconv.Itoa(pool.Quantity)
		if pool.Etcd != "true" && pool.Cp != "true" && pool.Wkr != "true" {
			return false, fmt.Errorf(`no roles selected for pool` + poolNum + `; at least one role is required`)
		}
		if pool.Quantity <= 0 {
			return false, fmt.Errorf(`invalid quantity specified for pool` + poolNum + `; quantity must be greater than 0`)
		}
		poolConfig = poolConfig + components.AKSNodePoolPrefix + poolNum + components.AKSNodePoolBody + quantity + components.AKSNodePoolBody2 + k8sVersion + components.AKSNodePoolSuffix
		num = num + 1
	}
	config = config + components.AKSClusterPrefix + k8sVersion + components.AKSClusterSpecs1 + poolConfig + components.AKSClusterSuffix
	_, err = file.WriteString(config)

	if err != nil {
		return false, err
	}
	return true, nil
}
