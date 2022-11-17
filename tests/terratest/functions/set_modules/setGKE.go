package functions

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rancher/rancher/tests/terratest/components"
	"github.com/rancher/rancher/tests/terratest/models"
)

func SetGKE(module string, k8sVersion string, nodePools []models.Nodepool, config string, file *os.File) (done bool, err error) {
	config = config + components.GkeCloudCredentials
	poolConfig := ``
	num := 1
	for _, pool := range nodePools {
		poolNum := strconv.Itoa(num)
		quantity := strconv.Itoa(pool.Quantity)
		max_pods_contraint := strconv.Itoa(pool.MaxPodsContraint)
		if pool.Quantity <= 0 {
			return false, fmt.Errorf(`invalid quantity specified for pool` + poolNum + `; quantity must be greater than 0`)
		}
		poolConfig = poolConfig + components.GKENodePoolPrefix + quantity + components.GKENodePoolSpecs1 + max_pods_contraint + components.GKENodePoolSpecs2 + poolNum + components.GKENodePoolSpecs3 + k8sVersion + components.GKENodePoolSuffix
		num = num + 1
	}
	config = config + components.GKEClusterPrefix + k8sVersion + components.GKEClusterSpecs + poolConfig + components.GKEClusterSuffix
	_, err = file.WriteString(config)

	if err != nil {
		return false, err
	}
	return true, nil
}
