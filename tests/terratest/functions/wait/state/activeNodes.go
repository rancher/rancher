package functions

import (
	"testing"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func ActiveNodes(t *testing.T, client *rancher.Client, clusterID string) (done bool, err error) {
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
					t.Logf("Failed to locate cluster and grab client specs. Error: %v", err)
					return false, err
				}

				if n.State == "active" {
					return true, nil
				}

				t.Logf("Failed to instantiate wait poll.")
				return false, nil
			})
			require.NoError(t, err)

		}
	}

	return true, nil
}
