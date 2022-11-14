package functions

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rancher/rancher/tests/terratest/components"
	"github.com/rancher/rancher/tests/terratest/models"
)

func SetEKS(module string, k8sVersion string, nodePools []models.Nodepool, config string, file *os.File) (done bool, err error) {
	config = config + components.ResourceEC2CloudCredentials
	poolConfig := ``
	num := 1
	for _, pool := range nodePools {
		poolNum := strconv.Itoa(num)
		desired_size := strconv.Itoa(pool.DesiredSize)
		max_size := strconv.Itoa(pool.MaxSize)
		min_size := strconv.Itoa(pool.MinSize)
		if pool.DesiredSize <= 1 {
			return false, fmt.Errorf(`invalid quantity specified for pool` + poolNum + `; quantity must be greater than 1`)
		}
		poolConfig = poolConfig + components.EKSNodePoolPrefix + poolNum + components.EKSNodePoolBody + pool.InstanceType + components.EKSNodePoolBody1 + desired_size + components.EKSNodePoolBody2 + max_size + components.EKSNodePoolBody3 + min_size + components.EKSNodePoolSuffix
		num = num + 1
	}
	config = config + components.EKSClusterPrefix + k8sVersion + components.EKSClusterSpecs1 + poolConfig + components.EKSClusterSuffix
	_, err = file.WriteString(config)

	if err != nil {
		return false, err
	}
	return true, nil
}
