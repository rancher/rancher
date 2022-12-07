package upgrade

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/norman/types"
	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/bundledclusters"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
)

const (
	// isCheckingCurrentCluster is the boolean to decide whether validations should happen with a new GET request and cluster data or not
	isCheckingCurrentCluster = true
	// logMessageKubernetesVersion is the string message to log with the kubernetes versions validations' assertions
	logMessageKubernetesVersion = "Validating the current version is the upgraded one"
	// logMessageNodepoolVersion is the string message to log the with kubernetes nodepools versions validations' assertions
	logMessageNodepoolVersions = "Validating the current nodepools version is the upgraded one"
	// ConfigurationFileKey is used to parse the configuration of upgrade tests.
	ConfigurationFileKey = "upgradeInput"
	// localClusterID is a string to used ignore this cluster in comparisons
	localClusterID = "local"
)

type ClustersToUpgrade struct {
	Name             string `json:"name" yaml:"name" default:""`
	VersionToUpgrade string `json:"versionToUpgrade" yaml:"versionToUpgrade" default:""`
	isLatestVersion  bool
}

type Config struct {
	ClustersToUpgrade []ClustersToUpgrade `json:"clusters" yaml:"clusters" default:"[]"`
}

// getVersion is a test helper function to get kubernetes version of a cluster.
// if versionToUpgrade in the config provided, checks the value within version first. Returns the version as string.
func getVersion(t *testing.T, clusterName string, versions []string, isLatestVersion bool, versionToUpgrade string) (version *string) {
	t.Helper()

	if isLatestVersion {
		require.NotEmptyf(t, versions, "[%v]: Can't upgrade cluster kubernetes version, cause it's already the latest", clusterName)
		version = &versions[len(versions)-1]
	} else if !isLatestVersion && versionToUpgrade != "" {
		require.Containsf(t, versions, versionToUpgrade, "[%v]: Specified version doesn't exist", clusterName)
		version = &versionToUpgrade
	}

	return
}

// validateKubernetesVersions is a test helper function to validate kubernetes version of a cluster.
// if isUsingCluster provided, checks with a new get method to validate with updated cluster.
// if isUsingCluster not provided, checks payload.
func validateKubernetesVersions(t *testing.T, client *rancher.Client, bc *bundledclusters.BundledCluster, versionToUpgrade *string, isUsingCurrentCluster bool) {
	t.Helper()

	cluster, err := bc.Get(client)
	require.NoErrorf(t, err, "[%v]: Error occured while validating kubernetes version", bc.Meta.Name)

	if isUsingCurrentCluster {
		cluster = bc
	}

	switch cluster.Meta.Provider {
	case clusters.KubernetesProviderRKE:
		assert.Equalf(t, *versionToUpgrade, cluster.V3.RancherKubernetesEngineConfig.Version, "[%v]: %v", cluster.Meta.Name, logMessageKubernetesVersion)
	case clusters.KubernetesProviderRKE2:
		if cluster.Meta.IsImported {
			clusterSpec := &apiv1.ClusterSpec{}
			err = v1.ConvertToK8sType(cluster.V1.Spec, clusterSpec)
			require.NoError(t, err)

			assert.Equalf(t, *versionToUpgrade, clusterSpec.KubernetesVersion, "[%v]: %v", cluster.Meta.Name, logMessageKubernetesVersion)
		} else if !cluster.Meta.IsImported {
			assert.Equalf(t, *versionToUpgrade, cluster.V3.Rke2Config.Version, "[%v]: %v", cluster.Meta.Name, logMessageKubernetesVersion)
		}
	case clusters.KubernetesProviderK3S:
		if cluster.Meta.IsImported {
			clusterSpec := &apiv1.ClusterSpec{}
			err = v1.ConvertToK8sType(cluster.V1.Spec, clusterSpec)
			require.NoError(t, err)

			assert.Equalf(t, *versionToUpgrade, clusterSpec.KubernetesVersion, "[%v]: %v", cluster.Meta.Name, logMessageKubernetesVersion)
		} else if !cluster.Meta.IsImported {
			assert.Equalf(t, *versionToUpgrade, cluster.V3.K3sConfig.Version, "[%v]: %v", cluster.Meta.Name, logMessageKubernetesVersion)
		}
	case clusters.KubernetesProviderGKE:
		assert.Equalf(t, *versionToUpgrade, *cluster.V3.GKEConfig.KubernetesVersion, "[%v]: %v", cluster.Meta.Name, logMessageKubernetesVersion)
	case clusters.KubernetesProviderAKS:
		assert.Equalf(t, *versionToUpgrade, *cluster.V3.AKSConfig.KubernetesVersion, "[%v]: %v", cluster.Meta.Name, logMessageKubernetesVersion)
	case clusters.KubernetesProviderEKS:
		assert.Equalf(t, *versionToUpgrade, *cluster.V3.EKSConfig.KubernetesVersion, "[%v]: %v", cluster.Meta.Name, logMessageKubernetesVersion)
	}
}

// validateNodepoolVersions is a test helper function to validate node pool versions of a cluster.
// if isUsingCluster provided, checks with a new get method to validate with updated cluster.
// if isUsingCluster not provided, checks the payload.
func validateNodepoolVersions(t *testing.T, client *rancher.Client, bc *bundledclusters.BundledCluster, versionToUpgrade *string, isUsingCurrentCluster bool) {
	t.Helper()

	cluster, err := bc.Get(client)
	require.NoErrorf(t, err, "[%v]: Error occured while validating nodepool versions", bc.Meta.Name)

	if isUsingCurrentCluster {
		cluster = bc
	}

	switch cluster.Meta.Provider {
	case clusters.KubernetesProviderGKE:
		for i := range cluster.V3.GKEConfig.NodePools {
			assert.Equalf(t, *versionToUpgrade, *cluster.V3.GKEConfig.NodePools[i].Version, "[%v]: %v", cluster.Meta.Name, logMessageNodepoolVersions)
		}
	case clusters.KubernetesProviderAKS:
		for i := range cluster.V3.AKSConfig.NodePools {
			assert.Equal(t, *versionToUpgrade, *cluster.V3.AKSConfig.NodePools[i].OrchestratorVersion, "[%v]: %v", cluster.Meta.Name, logMessageNodepoolVersions)
		}
	case clusters.KubernetesProviderEKS:
		for i := range cluster.V3.EKSConfig.NodeGroups {
			assert.Equal(t, *versionToUpgrade, *cluster.V3.EKSConfig.NodeGroups[i].Version, "[%v]: %v", cluster.Meta.Name, logMessageNodepoolVersions)
		}
	}
}

// loadUpgradeConfig is a test helper function to get required slice of ClustersToUpgrade struct. Returns error if any.
// Contains two different config options to the upgrade kubernetes test:
//  1. A flag called “all” is only requirement, upgrades all clusters except local to the latest available
//  2. An additional config field called update with slice of ClustersToUpgrade struct,
//     for choosing some clusters to upgrade(version is optional, by default to the latest available)
func loadUpgradeConfig(client *rancher.Client) (clusters []ClustersToUpgrade, err error) {
	upgradeConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, upgradeConfig)

	isConfigEmpty := len(upgradeConfig.ClustersToUpgrade) == 0
	isFlagDeclared := client.Flags.GetValue(environmentflag.UpgradeAllClusters)

	if isConfigEmpty && !isFlagDeclared {
		return clusters, errors.Wrap(err, "config doesn't match the requirements")
	}

	isUpgradeAllClusters := isFlagDeclared && isConfigEmpty
	isUpgradeSomeClusters := !isConfigEmpty && !isUpgradeAllClusters

	if isUpgradeAllClusters {
		clusterList, err := client.Management.Cluster.List(&types.ListOpts{})
		if err != nil {
			return clusters, errors.Wrap(err, "couldn't list clusters")
		}

		for i, c := range clusterList.Data {
			isLocalCluster := c.ID == localClusterID
			if !isLocalCluster {
				cluster := new(ClustersToUpgrade)

				cluster.Name = clusterList.Data[i].Name
				cluster.isLatestVersion = true

				clusters = append(clusters, *cluster)
			}
		}
	} else if isUpgradeSomeClusters {
		for _, c := range upgradeConfig.ClustersToUpgrade {
			cluster := new(ClustersToUpgrade)

			cluster.Name = c.Name
			cluster.VersionToUpgrade = c.VersionToUpgrade

			isVersionFieldEmpty := cluster.VersionToUpgrade == ""
			if isVersionFieldEmpty {
				cluster.isLatestVersion = true
			}

			clusters = append(clusters, *cluster)
		}
	}

	return
}
