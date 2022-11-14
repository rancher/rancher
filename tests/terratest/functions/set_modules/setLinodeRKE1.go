package functions

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rancher/rancher/tests/terratest/components"
	"github.com/rancher/rancher/tests/terratest/models"
)

func SetLinodeRKE1(module string, k8sVersion string, nodePools []models.Nodepool, config string, file *os.File) (done bool, err error) {
	config = config + components.RKE1ClusterPrefix + k8sVersion + components.RKE1ClusterSuffix + components.RKE1LinodeNodeTemplate
	poolConfig := ``
	clusterSyncNodePoolIDs := `  node_pool_ids = [`
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
		poolConfig = poolConfig + components.RKE1NodePoolPrefix + poolNum + components.RKE1NodePoolSpecs1 + poolNum + components.RKE1NodePoolSpecs2 + poolNum + `-"` + components.RKE1NodePoolSpecs25 + quantity + components.RKE1NodePoolSpecs3 + pool.Cp + components.RKE1NodePoolSpecs4 + pool.Etcd + components.RKE1NodePoolSpecs5 + pool.Wkr + components.RKE1NodePoolSuffix
		if num != len(nodePools) {
			clusterSyncNodePoolIDs = clusterSyncNodePoolIDs + `rancher2_node_pool.pool` + poolNum + `.id, `
		}
		if num == len(nodePools) {
			clusterSyncNodePoolIDs = clusterSyncNodePoolIDs + `rancher2_node_pool.pool` + poolNum + `.id]`
		}
		num = num + 1
	}
	config = config + poolConfig + components.RKE1ClusterSyncPrefix + clusterSyncNodePoolIDs + components.RKE1ClusterSyncSuffix

	_, err = file.WriteString(config)

	if err != nil {
		return false, err
	}
	return true, nil
}
