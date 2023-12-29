package eks

import (
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/sirupsen/logrus"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	active = "active"
)

// updateNodePoolQuantity is a helper method that will update the node pool with the desired quantity.
func updateNodePoolQuantity(client *rancher.Client, cluster *management.Cluster, nodePool *NodeGroupConfig) (*management.Cluster, error) {
	clusterResp, err := client.Management.Cluster.ByID(cluster.ID)
	if err != nil {
		return nil, err
	}

	var eksConfig = clusterResp.EKSConfig
	*eksConfig.NodeGroups[0].DesiredSize += *nodePool.DesiredSize

	eksHostCluster := &management.Cluster{
		DockerRootDir:          "/var/lib/docker",
		EKSConfig:              eksConfig,
		EnableClusterAlerting:  clusterResp.EnableClusterAlerting,
		EnableNetworkPolicy:    clusterResp.EnableNetworkPolicy,
		Labels:                 clusterResp.Labels,
		Name:                   clusterResp.Name,
		WindowsPreferedCluster: clusterResp.WindowsPreferedCluster,
	}

	logrus.Infof("Scaling the node group to %v total nodes", *eksConfig.NodeGroups[0].DesiredSize)
	updatedCluster, err := client.Management.Cluster.Update(clusterResp, eksHostCluster)
	if err != nil {
		return nil, err
	}

	err = kwait.Poll(500*time.Millisecond, 10*time.Minute, func() (done bool, err error) {
		clusterResp, err := client.Management.Cluster.ByID(updatedCluster.ID)
		if err != nil {
			return false, err
		}

		if clusterResp.State == active && clusterResp.NodeCount == *eksConfig.NodeGroups[0].DesiredSize {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return updatedCluster, nil
}

// ScalingEKSNodePoolsNodes is a helper function that tests scaling of an EKS node pool by adding a new one and then deleting it.
func ScalingEKSNodePoolsNodes(client *rancher.Client, cluster *management.Cluster, nodePool *NodeGroupConfig) (*management.Cluster, error) {
	updatedCluster, err := updateNodePoolQuantity(client, cluster, nodePool)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Node group has been scaled!")

	return updatedCluster, nil
}
