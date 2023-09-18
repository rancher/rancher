package scaling

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/sirupsen/logrus"
)

const (
	clusterNameLabel = "cluster.x-k8s.io/cluster-name"
	workloadNS       = "default"
	clusterNS        = "fleet-default"
)

func validateNodeCount(client *rancher.Client, clusterID string, nodeType string, initialNodeCount int) (bool, error) {
	if nodeType == "worker" {
		workerNodes, err := client.Management.Node.List(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId": clusterID,
				"worker":    true,
			},
		})
		if err != nil {
			return false, err
		}
		return len(workerNodes.Data) == initialNodeCount+1, err

	} else if nodeType == "controlPlane" {
		controlPlaneNodes, err := client.Management.Node.List(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId":    clusterID,
				"controlplane": true,
			},
		})
		if err != nil {
			return false, err
		}
		return len(controlPlaneNodes.Data) == initialNodeCount+1, err

	} else {
		etcdNodes, err := client.Management.Node.List(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId": clusterID,
				"etcd":      true,
			},
		})
		if err != nil {
			return false, err
		}
		return len(etcdNodes.Data) == initialNodeCount+2, err
	}
}

func deleteRKE2CustomClusterNode(client *rancher.Client, nodesToDelete []*nodes.Node, clusterID string) error {
	nodes, err := client.Management.Node.List(&types.ListOpts{Filters: map[string]interface{}{
		"clusterId": clusterID,
	}})
	if err != nil {
		return err
	}
	for _, nodeToDelete := range nodesToDelete {
		for _, node := range nodes.Data {
			if node.ExternalIPAddress == nodeToDelete.PublicIPAddress {
				machineToDelete, err := client.Steve.SteveType("cluster.x-k8s.io.machine").ByID(clusterNS + "/" + node.Annotations["cluster.x-k8s.io/machine"])
				if err != nil {
					return err
				}
				clusterName, err := clusters.GetClusterNameByID(client, clusterID)
				if err != nil {
					return err
				}
				if machineToDelete.Labels[clusterNameLabel] == clusterName {
					logrus.Info("deleting node...")
					err = client.Steve.SteveType("cluster.x-k8s.io.machine").Delete(machineToDelete)
					if err != nil {
						return err
					}
					break
				}
			}
		}
	}
	return err
}
