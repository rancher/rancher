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

func ScaleDown(t *testing.T, client *rancher.Client, clusterID string) (done bool, err error) {
	clusterConfig := new(terratest.TerratestConfig)
	config.LoadConfig("terratest", clusterConfig)

	wait_err := wait.Poll(100*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		cluster, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		if err != nil {
			t.Logf("Failed to locate cluster and grab client specs. Error: %v", err)
			return false, err
		}

		if cluster.NodeCount == clusterConfig.ScaledDownNodeCount {
			t.Logf("Successfully scaled down cluster to %v nodes", clusterConfig.ScaledDownNodeCount)
			return true, nil
		}

		t.Logf("Waiting for cluster to scale down...")
		return false, nil
	})
	require.NoError(t, wait_err)

	if wait_err != nil {
		t.Logf("Failed to instantiate scale down wait poll.")
		return false, wait_err
	}

	return true, nil
}
