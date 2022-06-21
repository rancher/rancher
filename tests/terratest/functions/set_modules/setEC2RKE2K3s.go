package functions

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rancher/rancher/tests/terratest/components"
	"github.com/rancher/rancher/tests/terratest/models"
)

func SetEC2RKE2K3s(module string, k8sVersion string, nodePools []models.Nodepool, config string, file *os.File) (done bool, err error) {
	config = config + components.DataCloudCredentials + components.V2EC2MachineConfig
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
		poolConfig = poolConfig + components.V2MachinePoolsPrefix + poolNum + components.V2MachinePoolsSpecs1 + pool.Cp + components.V2MachinePoolsSpecs2 + pool.Etcd + components.V2MachinePoolsSpecs3 + pool.Wkr + components.V2MachinePoolsSpecs4 + quantity + components.V2MachinePoolsSuffix
		num = num + 1
	}
	config = config + components.V2ClusterPrefix + k8sVersion + components.V2ClusterBody + poolConfig + components.V2ClusterSuffix

	_, err = file.WriteString(config)

	if err != nil {
		return false, err
	}
	return true, nil
}
