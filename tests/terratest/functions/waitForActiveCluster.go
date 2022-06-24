package functions

import (
	"testing"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/stretchr/testify/require"
)

func WaitForActiveCluster(t *testing.T, client *rancher.Client, clusterID string) {
	time.Sleep(10 * time.Second)

	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	state := cluster.State

	for state != "active" {
		for state != "updating" {
			time.Sleep(10 * time.Second)

			cluster, err = client.Management.Cluster.ByID(clusterID)
			require.NoError(t, err)

			state = cluster.State
		}
		time.Sleep(10 * time.Second)

		cluster, err = client.Management.Cluster.ByID(clusterID)
		require.NoError(t, err)

		state = cluster.State
	}
	time.Sleep(10 * time.Second)
}
