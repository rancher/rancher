package nodes

import (
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	active = "active"
)

// IsNodeReady is a helper method that will loop and check if the node is ready in the RKE1 cluster.
// It will return an error if the node is not ready after set amount of time.
func IsNodeReady(client *rancher.Client, ClusterID string) error {
	err := wait.Poll(500*time.Millisecond, 30*time.Minute, func() (bool, error) {
		nodes, err := client.Management.Node.ListAll(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId": ClusterID,
			},
		})
		if err != nil {
			return false, err
		}

		for _, node := range nodes.Data {
			node, err := client.Management.Node.ByID(node.ID)
			if err != nil {
				return false, err
			}

			if node.State == active {
				logrus.Infof("All nodes in the cluster are in an active state!")
				return true, nil
			}

			return false, nil
		}

		return false, nil
	})

	return err
}
