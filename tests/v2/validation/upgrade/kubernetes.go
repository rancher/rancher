package upgrade

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/bundledclusters"
	kcluster "github.com/rancher/rancher/tests/framework/extensions/kubeapi/cluster"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	local = "local"

	// isCheckingCurrentCluster is the boolean to decide whether validations should happen with a new GET request and cluster data or not
	isCheckingCurrentCluster = true
	// logMessageKubernetesVersion is the string message to log with the kubernetes versions validations' assertions
	logMessageKubernetesVersion = "Validating the current version is the upgraded one"
	// logMessageNodepoolVersion is the string message to log the with kubernetes nodepools versions validations' assertions
	logMessageNodepoolVersions = "Validating the current nodepools version is the upgraded one"
)

func waitUntilLocalStable(client *rancher.Client, clusterName string) error {
	return kwait.Poll(10*time.Second, 75*time.Minute, func() (bool, error) {
		isConnected, err := client.IsConnected()
		if err != nil {
			logrus.Errorf("[connection stable]: %v", err)
			return false, nil
		}
		logrus.Infof("[connection stable]: %v", isConnected)

		if isConnected {
			ready, err := kcluster.IsClusterActive(client, clusterName)
			if err != nil {
				logrus.Errorf("[cluster ready]: %v", err)
				return false, nil
			}
			logrus.Infof("[cluster ready]: %v", ready)

			return ready, nil
		}

		return false, nil
	})
}

// getVersion is a test helper function to get kubernetes version of a cluster.
// if versionToUpgrade in the config provided, checks the value within version first. Returns the version as string.
func getVersion(t *testing.T, clusterName string, versions []string, isLatestVersion bool, versionToUpgrade string) (version *string) {
	t.Helper()

	if len(versions) == 0 {
		t.Skipf("Kubernetes upgrade is not possible for [%v] cause the version pool is empty", clusterName)
	}

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
	require.NoErrorf(t, err, "[%v]: Error occurred while validating kubernetes version", bc.Meta.Name)

	if isUsingCurrentCluster {
		cluster = bc
	}

	switch cluster.Meta.Provider {
	case clusters.KubernetesProviderRKE:
		if cluster.Meta.IsImported {
			assert.Equalf(t, *versionToUpgrade, cluster.V3.RancherKubernetesEngineConfig.Version, "[%v]: %v", cluster.Meta.Name, logMessageKubernetesVersion)
		} else if !cluster.Meta.IsImported {
			assert.Equalf(t, *versionToUpgrade, cluster.V3.RancherKubernetesEngineConfig.Version, "[%v]: %v", cluster.Meta.Name, logMessageKubernetesVersion)
		}
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
	require.NoErrorf(t, err, "[%v]: Error occurred while validating nodepool versions", bc.Meta.Name)

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
