package functions

import (
	"testing"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func WaitingOrUpdating(t *testing.T, client *rancher.Client, clusterID string) (done bool, err error) {
	wait_err := wait.Poll(100*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		cluster, err := client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		if err != nil {
			t.Logf("Failed to locate cluster and grab client specs. Error: %v", err)
			return false, err
		}

		if cluster.State == "waiting" {
			t.Logf("Cluster is now in a waiting state.")
			return true, nil
		}

		if cluster.State == "updating" {
			t.Logf("Cluster is now in an updating state.")
			return true, nil
		}

		t.Logf("Waiting for cluster nodes to be in waiting or updating state...")
		return false, nil
	})
	require.NoError(t, wait_err)

	if wait_err != nil {
		t.Logf("Failed to instantiate waiting or updating wait poll.")
		return false, wait_err
	}

	return true, nil
}