package functions

import (
	"testing"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func WaitForActiveCluster(t *testing.T, client *rancher.Client, clusterID string, module string) {

	if module == "aks" || module == "eks" {
		// Check for updating/waiting cluster state
		err := wait.Poll(100*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
			cluster, err := client.Management.Cluster.ByID(clusterID)
			require.NoError(t, err)

			if err != nil {
				return false, err
			}

			if cluster.State == "updating" {
				return true, nil
			}

			if cluster.State == "waiting" {
				return true, nil
			}

			return false, nil
		})
		require.NoError(t, err)

		// Check for active cluster state
		err = wait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
			cluster, err := client.Management.Cluster.ByID(clusterID)
			require.NoError(t, err)

			if err != nil {
				return false, err
			}

			if cluster.State == "active" {
				return true, nil
			}

			return false, nil
		})
		require.NoError(t, err)
		time.Sleep(10 * time.Second)
	}

	if module == "ec2_k3s" || module == "ec2_rke1" || module == "ec2_rke2" || module == "linode_k3s" || module == "linode_rke1" || module == "linode_rke2" {
		// Check cluster state is waiting/updating
		err := wait.Poll(100*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
			cluster, err := client.Management.Cluster.ByID(clusterID)
			require.NoError(t, err)

			if err != nil {
				return false, err
			}

			if cluster.State == "waiting" {
				return true, nil
			}

			if cluster.State == "updating" {
				return true, nil
			}

			return false, nil
		})
		require.NoError(t, err)

		// Check cluster state is active
		err = wait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
			cluster, err := client.Management.Cluster.ByID(clusterID)
			require.NoError(t, err)

			if err != nil {
				return false, err
			}

			if cluster.State == "active" {
				return true, nil
			}

			return false, nil
		})
		require.NoError(t, err)

		// Check individal node state is active
		nodes, err := client.Management.Node.List(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId": clusterID,
			},
		})
		require.NoError(t, err)

		for _, node := range nodes.Data {

			if node.State != "active" {
				err = wait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
					n, err := client.Management.Node.ByID(node.ID)
					require.NoError(t, err)

					if err != nil {
						return false, err
					}

					if n.State == "active" {
						return true, nil
					}

					return false, nil
				})
				require.NoError(t, err)

			}
		}

		// Confirm cluster state active
		err = wait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
			cluster, err := client.Management.Cluster.ByID(clusterID)
			require.NoError(t, err)

			if err != nil {
				return false, err
			}

			if cluster.State == "active" {
				return true, nil
			}

			return false, nil
		})
		require.NoError(t, err)
		time.Sleep(10 * time.Second)
	}
}
