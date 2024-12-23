package upgradeinput

import (
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/environmentflag"
)

// loadUpgradeKubernetesConfig is a test helper function to get required slice of Clusters struct. Returns error if any.
// Contains two different config options to the upgrade kubernetes test:
//  1. A flag called “all” is only requirement, upgrades all clusters except local to the latest available
//  2. An additional config field called update with slice of Clusters struct,
//     for choosing some clusters to upgrade (version is optional, by default, empty string skips the test
//     and "latest" upgrades to the latest available.)
func LoadUpgradeKubernetesConfig(client *rancher.Client) (clusters []Cluster, err error) {
	upgradeConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, upgradeConfig)

	isConfigEmpty := len(upgradeConfig.Clusters) == 0
	isFlagDeclared := client.Flags.GetValue(environmentflag.KubernetesUpgradeAllClusters)

	if isConfigEmpty && !isFlagDeclared {
		return clusters, errors.Wrap(err, "config doesn't match the requirements")
	}

	isUpgradeAllClusters := isFlagDeclared && isConfigEmpty
	isUpgradeSomeClusters := !isConfigEmpty && !isUpgradeAllClusters

	if isUpgradeAllClusters {
		clusterList, err := listDownstreamClusters(client)
		if err != nil {
			return clusters, errors.Wrap(err, "couldn't list clusters")
		}

		for i := range clusterList {
			cluster := new(Cluster)

			cluster.Name = clusterList[i]
			ingress := false
			chart := false
			cluster.FeaturesToTest = Features{
				Ingress: &ingress,
				Chart:   &chart,
			}

			clusters = append(clusters, *cluster)
		}
	} else if isUpgradeSomeClusters {
		for _, c := range upgradeConfig.Clusters {
			cluster := new(Cluster)

			cluster.Name = c.Name
			cluster.ProvisioningInput = c.ProvisioningInput
			cluster.FeaturesToTest = c.FeaturesToTest
			cluster.VersionToUpgrade = c.VersionToUpgrade

			clusters = append(clusters, *cluster)
		}
	}

	return
}

func listDownstreamClusters(client *rancher.Client) (clusterNames []string, err error) {
	clusterList, err := client.Management.Cluster.List(&types.ListOpts{})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't list downstream clusters")
	}

	for i, c := range clusterList.Data {
		isLocalCluster := c.ID == localClusterID
		if !isLocalCluster {
			clusterNames = append(clusterNames, clusterList.Data[i].Name)
		}
	}

	return
}
