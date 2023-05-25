package functions

import (
	"testing"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/v2/validation/terratest"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func KubernetesUpgrade(t *testing.T, client *rancher.Client, clusterID string, module string) (done bool, err error) {
	clusterConfig := new(terratest.TerratestConfig)
	config.LoadConfig("terratest", clusterConfig)

	if module == "aks" || module == "gke" {
		expectedUpgradedKubernetesVersion := `v` + clusterConfig.UpgradedKubernetesVersion
		wait_err := wait.Poll(100*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
			cluster, err := client.Management.Cluster.ByID(clusterID)
			require.NoError(t, err)

			if err != nil {
				t.Logf("Failed to locate cluster and grab client specs. Error: %v", err)
				return false, err
			}

			if cluster.Version.GitVersion == expectedUpgradedKubernetesVersion {
				t.Logf("Successfully updated kubernetes version to %v", expectedUpgradedKubernetesVersion)
				return true, nil
			}

			t.Logf("Waiting for cluster to upgrade kubernetes version...")
			return false, nil
		})
		require.NoError(t, wait_err)

		if wait_err != nil {
			t.Logf("Failed to instantiate kubernetes upgrade wait poll.")
			return false, wait_err
		}
	}

	if module == "ec2_rke1" || module == "linode_rke1" {
		expectedUpgradedKubernetesVersion := clusterConfig.UpgradedKubernetesVersion[:len(clusterConfig.UpgradedKubernetesVersion)-11]
		wait_err := wait.Poll(100*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
			cluster, err := client.Management.Cluster.ByID(clusterID)
			require.NoError(t, err)

			if err != nil {
				t.Logf("Failed to locate cluster and grab client specs. Error: %v", err)
				return false, err
			}

			if cluster.Version.GitVersion == expectedUpgradedKubernetesVersion {
				t.Logf("Successfully updated kubernetes version to %v", expectedUpgradedKubernetesVersion)
				return true, nil
			}

			t.Logf("Waiting for cluster to upgrade kubernetes version...")
			return false, nil
		})
		require.NoError(t, wait_err)

		if wait_err != nil {
			t.Logf("Failed to instantiate kubernetes upgrade wait poll.")
			return false, wait_err
		}
	}

	if module == "ec2_k3s" || module == "ec2_rke2" || module == "linode_k3s" || module == "linode_rke2" {
		wait_err := wait.Poll(100*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
			cluster, err := client.Management.Cluster.ByID(clusterID)
			require.NoError(t, err)

			if err != nil {
				t.Logf("Failed to locate cluster and grab client specs. Error: %v", err)
				return false, err
			}

			if cluster.Version.GitVersion == clusterConfig.UpgradedKubernetesVersion {
				t.Logf("Successfully updated kubernetes version to %v", clusterConfig.UpgradedKubernetesVersion)
				return true, nil
			}

			t.Logf("Waiting for cluster to upgrade kubernetes version...")
			return false, nil
		})
		require.NoError(t, wait_err)

		if wait_err != nil {
			t.Logf("Failed to instantiate kubernetes upgrade wait poll.")
			return false, wait_err
		}
	}

	return true, nil
}