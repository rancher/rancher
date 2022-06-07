package functions

import (
	"fmt"
	"os"
	"strconv"

	"github.com/josh-diamond/rancher-terratest/components"
	"github.com/josh-diamond/rancher-terratest/models"
)

func SetConfigTF(module string, k8sVersion string, nodePools []models.Nodepool) bool {

	config := components.RequiredProviders + components.Provider

	if module == "aks" || module == "rke1" || module == "rke2" || module == "k3s" {
		f, err := os.Create("../../modules/" + module + "/main.tf")
		if err != nil {
			fmt.Println(err)
		}

		defer f.Close()

		switch {
		case module == "aks":
			config = config + components.AzureCloudCredentials
			poolConfig := ``
			num := 1
			for _, pool := range nodePools {
				poolNum := strconv.Itoa(num)
				quantity := strconv.Itoa(pool.Quantity)
				if pool.Etcd != "true" && pool.Cp != "true" && pool.Wkr != "true" {
					fmt.Println(`No roles selected for pool` + poolNum + `; At least one role is required`)
					return false
				}
				if pool.Quantity <= 0 {
					fmt.Println(`Invalid quantity specified for pool` + poolNum + `; Quantity must be greater than 0`)
					return false
				}
				poolConfig = poolConfig + components.AKSNodePoolPrefix + poolNum + components.AKSNodePoolBody + quantity + components.AKSNodePoolBody2 + k8sVersion + components.AKSNodePoolSuffix
				num = num + 1
			}
			config = config + components.AKSClusterPrefix + k8sVersion + components.AKSClusterSpecs1 + poolConfig + components.AKSClusterSuffix
			_, err = f.WriteString(config)

			if err != nil {
				fmt.Println(err)
				return false
			}
			return true

		case module == "rke1":
			config = config + components.ResourceEC2CloudCredentials + components.RKE1ClusterPrefix + k8sVersion + components.RKE1ClusterSuffix + components.RKE1NodeTemplate
			poolConfig := ``
			num := 1
			for _, pool := range nodePools {
				poolNum := strconv.Itoa(num)
				quantity := strconv.Itoa(pool.Quantity)
				if pool.Etcd != "true" && pool.Cp != "true" && pool.Wkr != "true" {
					fmt.Println(`No roles selected for pool` + poolNum + `; At least one role is required`)
					return false
				}
				if pool.Quantity <= 0 {
					fmt.Println(`Invalid quantity specified for pool` + poolNum + `; Quantity must be greater than 0`)
					return false
				}
				poolConfig = poolConfig + components.RKE1NodePoolPrefix + poolNum + components.RKE1NodePoolSpecs1 + poolNum + components.RKE1NodePoolSpecs2 + quantity + components.RKE1NodePoolSpecs3 + pool.Cp + components.RKE1NodePoolSpecs4 + pool.Etcd + components.RKE1NodePoolSpecs5 + pool.Wkr + components.RKE1NodePoolSuffix
				num = num + 1
			}
			config = config + poolConfig

			_, err = f.WriteString(config)

			if err != nil {
				fmt.Println(err)
				return false
			}
			return true

		case module == "rke2" || module == "k3s":
			config = config + components.DataEC2CloudCredentials + components.V2MachineConfig
			poolConfig := ``
			num := 1
			for _, pool := range nodePools {
				poolNum := strconv.Itoa(num)
				quantity := strconv.Itoa(pool.Quantity)
				if pool.Etcd != "true" && pool.Cp != "true" && pool.Wkr != "true" {
					fmt.Println(`No roles selected for pool` + poolNum + `; At least one role is required`)
					return false
				}
				if pool.Quantity <= 0 {
					fmt.Println(`Invalid quantity specified for pool` + poolNum + `; Quantity must be greater than 0`)
					return false
				}
				poolConfig = poolConfig + components.V2MachinePoolsPrefix + poolNum + components.V2MachinePoolsSpecs1 + pool.Cp + components.V2MachinePoolsSpecs2 + pool.Etcd + components.V2MachinePoolsSpecs3 + pool.Wkr + components.V2MachinePoolsSpecs4 + quantity + components.V2MachinePoolsSuffix
				num = num + 1
			}
			config = config + components.V2ClusterPrefix + k8sVersion + components.V2ClusterBody + poolConfig + components.V2ClusterSuffix

			_, err = f.WriteString(config)

			if err != nil {
				fmt.Println(err)
				return false
			}
			return true

		default:
			return false
		}
	}
	fmt.Println(`Invalid module provided`)
	return false
}
