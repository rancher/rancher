package upgrades

import (
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	aks "github.com/rancher/rancher/tests/framework/extensions/clusters/aks"
	"github.com/sirupsen/logrus"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

// UpgradeK8SVersion is a helper function that upgrades the AKS cluster's k8s version
func UpgradeK8SVersion(client *rancher.Client, cluster *management.Cluster, displayName, cloudCredentialID string) (*management.Cluster, error) {
	logrus.Infof("======================================")
	logrus.Infof("TEST CASE: Upgrading K8s Version")
	logrus.Infof("======================================")

	version_1_24_6 := "1.24.6"

	if cluster.AKSConfig.KubernetesVersion != &version_1_24_6 {
		aksHostCluster := aks.AKSHostClusterConfig(displayName, cloudCredentialID)
		aksHostCluster.KubernetesVersion = &version_1_24_6
		aksHostCluster.NodePools = cluster.AKSConfig.NodePools

		for node := range aksHostCluster.NodePools {
			aksHostCluster.NodePools[node].OrchestratorVersion = &version_1_24_6
		}

		updatedPoolsCluster := &management.Cluster{
			AKSConfig:               aksHostCluster,
			DockerRootDir:           "/var/lib/docker",
			EnableClusterAlerting:   cluster.EnableClusterAlerting,
			EnableClusterMonitoring: cluster.EnableClusterMonitoring,
			EnableNetworkPolicy:     cluster.EnableNetworkPolicy,
			Labels:                  cluster.Labels,
			Name:                    cluster.Name,
			WindowsPreferedCluster:  cluster.WindowsPreferedCluster,
		}

		logrus.Infof("Upgrading AKS cluster and nodes K8s version to v%s...", *updatedPoolsCluster.AKSConfig.KubernetesVersion)
		newCluster, err := client.Management.Cluster.Update(cluster, updatedPoolsCluster)
		if err != nil {
			return nil, err
		}

		err = kwait.Poll(500*time.Millisecond, 60*time.Minute, func() (done bool, err error) {
			client, err = client.ReLogin()
			if err != nil {
				return false, err
			}

			clusterResp, err := client.Management.Cluster.ByID(newCluster.ID)
			if err != nil {
				return false, err
			}

			if *clusterResp.AKSStatus.UpstreamSpec.KubernetesVersion != *updatedPoolsCluster.AKSConfig.KubernetesVersion && clusterResp.State != "active" {
				return false, err
			} else if *clusterResp.AKSStatus.UpstreamSpec.KubernetesVersion == *updatedPoolsCluster.AKSConfig.KubernetesVersion && clusterResp.State == "active" {
				logrus.Infof("AKS cluster and nodes have been upgraded to K8s v%s!", *updatedPoolsCluster.AKSConfig.KubernetesVersion)
				return true, nil
			}

			return false, nil
		})

		if err != nil {
			return nil, err
		}

		return newCluster, nil

	} else {
		logrus.Infof("K8s version is already at v%s, skipping upgrade...", *cluster.AKSConfig.KubernetesVersion)
	}

	return cluster, nil
}
