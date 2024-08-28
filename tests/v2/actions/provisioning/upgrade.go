package provisioning

import (
	"fmt"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/bundledclusters"
)

// UpgradeClusterK8sVersion upgrades the cluster to the specified version
func UpgradeClusterK8sVersion(client *rancher.Client, clusterName *string, upgradeVersion *string) (*bundledclusters.BundledCluster, error) {
	clusterMeta, err := clusters.NewClusterMeta(client, *clusterName)
	if err != nil {
		return nil, err
	}
	if clusterMeta == nil {
		return nil, fmt.Errorf("cluster %s not found", *clusterName)
	}

	initCluster, err := bundledclusters.NewWithClusterMeta(clusterMeta)
	if err != nil {
		return nil, err
	}

	cluster, err := initCluster.Get(client)
	if err != nil {
		return nil, err
	}

	updatedCluster, err := cluster.UpdateKubernetesVersion(client, upgradeVersion)
	if err != nil {
		return nil, err
	}

	err = clusters.WaitClusterToBeUpgraded(client, clusterMeta.ID)
	if err != nil {
		return nil, err
	}
	return updatedCluster, nil
}
