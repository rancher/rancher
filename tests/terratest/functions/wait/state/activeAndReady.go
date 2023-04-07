package functions

import (
	"testing"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func ActiveAndReady(t *testing.T, client *rancher.Client, clusterID string) (done bool, err error) {
	wait_err := wait.Poll(100*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		cluster, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		if err != nil {
			t.Logf("Failed to locate cluster and grab client specs. Error: %v", err)
			return false, err
		}

		if cluster.State == "active" && cluster.Conditions[0].Status == "True" {
			t.Logf("Cluster is now active and ready.")
			return true, nil
		}
		
		t.Logf("Waiting for cluster to be in an active and ready state...")
		return false, nil
	})
	require.NoError(t, wait_err)

	if wait_err != nil {
		t.Logf("Failed to instantiate active and ready wait poll.")
		return false, wait_err
	}

	return true, nil
}
