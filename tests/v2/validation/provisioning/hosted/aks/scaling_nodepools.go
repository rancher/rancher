package provisioning

import (
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/aks"
	np "github.com/rancher/rancher/tests/framework/extensions/clusters/aks/resources"
	"github.com/sirupsen/logrus"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

// ScalingAKSNodePools is a helper function that tests scaling of an AKS node pool by adding a new one and then deleting it.
func ScalingAKSNodePools(client *rancher.Client, oldCluster *management.Cluster, displayName string, cloudCredential *cloudcredentials.CloudCredential) (*management.Cluster, error) {
	aksHostCluster := aks.HostClusterConfig(displayName, cloudCredential.ID)
	nodePool, err := np.CreateNodePool(aksHostCluster)
	if err != nil {
		return nil, err
	}

	nodePoolName := "scaling"
	nodePool.Name = &nodePoolName

	aksHostCluster.NodePools = append(aksHostCluster.NodePools, *nodePool)

	updatedCluster := &management.Cluster{
		AKSConfig:               aksHostCluster,
		DockerRootDir:           "/var/lib/docker",
		EnableClusterAlerting:   oldCluster.EnableClusterAlerting,
		EnableClusterMonitoring: oldCluster.EnableClusterMonitoring,
		EnableNetworkPolicy:     oldCluster.EnableNetworkPolicy,
		Labels:                  oldCluster.Labels,
		Name:                    oldCluster.Name,
		WindowsPreferedCluster:  oldCluster.WindowsPreferedCluster,
	}

	logrus.Infof("Adding new AKS node pool...")
	cluster, err := client.Management.Cluster.Update(oldCluster, updatedCluster)
	if err != nil {
		return nil, err
	}

	err = kwait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}

		clusterResp, err := client.Management.Cluster.ByID(cluster.ID)
		if err != nil {
			return false, err
		}

		if clusterResp.NodeCount > *nodePool.Count && clusterResp.State == "active" {
			logrus.Infof("Node pool successfully added!")
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	updatedCluster.AKSConfig.NodePools = oldCluster.AKSConfig.NodePools

	logrus.Infof("Deleting new AKS node pool...")
	cluster, err = client.Management.Cluster.Update(cluster, updatedCluster)
	if err != nil {
		return nil, err
	}

	err = kwait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}

		clusterResp, err := client.Management.Cluster.ByID(cluster.ID)
		if err != nil {
			return false, err
		}

		if clusterResp.LinuxWorkerCount == *nodePool.Count && clusterResp.State == "active" {
			logrus.Infof("Node pool successfully deleted!")
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return cluster, err
}
