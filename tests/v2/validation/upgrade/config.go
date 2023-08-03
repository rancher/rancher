package upgrade

import (
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
)

type PSACT string

const (
	ConfigurationFileKey       = "upgradeInput" // ConfigurationFileKey is used to parse the configuration of upgrade tests.
	localClusterID             = "local"        // localClusterID is a string to used ignore this cluster in comparisons
	latestKey                  = "latest"       // latestKey is a string to determine automatically version pooling to the latest possible
	RancherPrivileged    PSACT = "rancher-privileged"
	RancherRestricted    PSACT = "rancher-restricted"
)

type Clusters struct {
	Name              string   `json:"name" yaml:"name" default:""`
	VersionToUpgrade  string   `json:"versionToUpgrade" yaml:"versionToUpgrade" default:""`
	PSACT             string   `json:"psact" yaml:"psact" default:""`
	FeaturesToTest    Features `json:"enabledFeatures" yaml:"enabledFeatures" default:""`
	isLatestVersion   bool
	isUpgradeDisabled bool
}

type Features struct {
	Chart   *bool `json:"chart" yaml:"chart" default:"false"`
	Ingress *bool `json:"ingress" yaml:"ingress" default:"false"`
}

type Config struct {
	Clusters []Clusters `json:"clusters" yaml:"clusters" default:"[]"`
}

// loadUpgradeKubernetesConfig is a test helper function to get required slice of ClustersToUpgrade struct. Returns error if any.
// Contains two different config options to the upgrade kubernetes test:
//  1. A flag called “all” is only requirement, upgrades all clusters except local to the latest available
//  2. An additional config field called update with slice of ClustersToUpgrade struct,
//     for choosing some clusters to upgrade (version is optional, by default, empty string skips the test
//     and "latest" upgrades to the latest available.)
func loadUpgradeKubernetesConfig(client *rancher.Client) (clusters []Clusters, err error) {
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
			cluster := new(Clusters)

			cluster.Name = clusterList[i]
			cluster.isLatestVersion = true

			clusters = append(clusters, *cluster)
		}
	} else if isUpgradeSomeClusters {
		for _, c := range upgradeConfig.Clusters {
			cluster := new(Clusters)

			cluster.Name = c.Name
			cluster.VersionToUpgrade = c.VersionToUpgrade
			cluster.PSACT = c.PSACT

			isVersionFieldLatest := cluster.VersionToUpgrade == latestKey
			if isVersionFieldLatest {
				cluster.isLatestVersion = true
			}

			isUpgradeDisabled := cluster.VersionToUpgrade == ""
			if isUpgradeDisabled {
				cluster.isUpgradeDisabled = true
			}

			clusters = append(clusters, *cluster)
		}
	}

	return
}

func loadUpgradeWorkloadConfig(client *rancher.Client) (clusters []Clusters, err error) {
	upgradeConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, upgradeConfig)

	isConfigEmpty := len(upgradeConfig.Clusters) == 0
	isFlagDeclared := client.Flags.GetValue(environmentflag.WorkloadUpgradeAllClusters)

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
			cluster := new(Clusters)

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
			cluster := new(Clusters)

			cluster.Name = c.Name
			cluster.FeaturesToTest = c.FeaturesToTest
			cluster.PSACT = c.PSACT

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
